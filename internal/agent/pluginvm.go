package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"

	"github.com/dxn111/EVOLV/internal/bus"
)

// Each plugin runs in its OWN goja runtime with a live event API:
//
//      evolv.on("evolution", (e) => {...})   react to any bus event, live
//      evolv.every(2000, () => {...})        real timers, live
//      evolv.log(...), evolv.toast("…")
//
// One goroutine per plugin owns its VM: it drains the bus-event queue and
// fires due timers, so handlers never race the plugin's own top-level code.

type pluginVM struct {
	name         string
	a            *Agent
	vm           *goja.Runtime
	mu           sync.Mutex
	events       chan bus.Event
	handlers     map[string][]goja.Callable
	timers       []timer
	perms        map[string]bool
	loadTimeout  time.Duration
	eventTimeout time.Duration
	stop         chan struct{}
	stopOnce     sync.Once
}

// Stop retires the plugin's scheduler. The VM is never reused after this; a
// reload builds a fresh sandbox. Idempotent so both LoadAll and evolve_plugin
// can call it without coordinating.
func (p *pluginVM) Stop() {
	p.stopOnce.Do(func() { close(p.stop) })
}

// SetEventTimeout sets the execution timeout for event handlers and timer callbacks.
func (p *pluginVM) SetEventTimeout(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.eventTimeout = d
}

// pluginCaps is every gateable capability a plugin can request.
var pluginCaps = []string{"http", "fs", "toast"}

// hasPerm reports whether the plugin was granted a capability. Core
// capabilities (log, on, every) are always on; gateable ones must be listed
// in the plugin's manifest perms.
func (p *pluginVM) hasPerm(cap string) bool { return p.perms[cap] }

// deny builds the permission-denied error a gated capability throws.
func deny(plugin, cap string) string {
	return fmt.Sprintf("plugin %q lacks permission %q — re-evolve it with perms:[%q]", plugin, cap, cap)
}

type timer struct {
	ms   int
	fn   goja.Callable
	next time.Time
}

const pluginEventQueue = 128

// buildPluginVM creates the sandboxed runtime and runs the plugin code.
// Callers holding JSRuntime.mu must not re-lock it. Does NOT start the loop.
func (r *JSRuntime) buildPluginVM(name, code string, perms []string) (*pluginVM, error) {
	vm := goja.New()
	vm.SetMaxCallStackSize(DefaultMaxCallStackSize)

	loadTimeout := r.pluginLoadTimeout
	if loadTimeout <= 0 {
		loadTimeout = DefaultPluginLoadTimeout
	}

	p := &pluginVM{
		name:         name,
		a:            r.a,
		vm:           vm,
		events:       make(chan bus.Event, pluginEventQueue),
		handlers:     map[string][]goja.Callable{},
		perms:        map[string]bool{},
		loadTimeout:  loadTimeout,
		eventTimeout: DefaultPluginEventTimeout,
		stop:         make(chan struct{}),
	}
	for _, c := range perms {
		p.perms[strings.TrimSpace(strings.ToLower(c))] = true
	}
	api := map[string]any{
		"log": func(args ...goja.Value) {
			parts := make([]string, len(args))
			for i, v := range args {
				parts[i] = v.String()
			}
			p.a.bus.Publish("plugin:log", map[string]string{"plugin": name, "text": strings.Join(parts, " ")})
		},
		"toast": func(msg string) {
			if !p.hasPerm("toast") {
				panic(vm.NewTypeError(deny(name, "toast")))
			}
			p.a.bus.Publish("plugin:toast", map[string]string{"plugin": name, "text": msg})
		},
		"workspace_dir": func() string {
			if !p.hasPerm("fs") {
				panic(vm.NewTypeError(deny(name, "fs")))
			}
			return p.a.ws.Root
		},
		"name": name,
		"http": func(url string) map[string]any {
			if !p.hasPerm("http") {
				panic(vm.NewTypeError(deny(name, "http")))
			}
			return httpFetch(url)
		},
		"on": func(event string, fn goja.Value) {
			if cb, ok := goja.AssertFunction(fn); ok {
				// stash the callable on the VM so it isn't garbage collected
				vm.Set("__h_"+event+"_"+time.Now().Format("150405.000000000"), cb)
				p.handlers[event] = append(p.handlers[event], cb)
			}
		},
		"every": func(ms int, fn goja.Value) {
			if ms < 50 {
				ms = 50
			}
			if cb, ok := goja.AssertFunction(fn); ok {
				vm.Set("__t_"+time.Now().Format("150405.000000000"), cb)
				p.timers = append(p.timers, timer{ms: ms, fn: cb, next: time.Now().Add(time.Duration(ms) * time.Millisecond)})
			}
		},
	}
	if err := vm.Set("evolv", api); err != nil {
		return nil, err
	}
	err := runWithTimeout(vm, loadTimeout, func() error {
		_, runErr := vm.RunString("(function(){\n" + code + "\n})();")
		return runErr
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// loop is the plugin's private scheduler: bus events in, timers out.
func (p *pluginVM) loop() {
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
		}
		// drain pending events (at most a few per tick)
		for i := 0; i < 8; i++ {
			select {
			case ev := <-p.events:
				p.fire(ev.Type, json.RawMessage(ev.Payload))
				continue
			default:
			}
			break
		}
		p.mu.Lock()
		now := time.Now()
		var due []timer
		for i := range p.timers {
			if !now.Before(p.timers[i].next) {
				p.timers[i].next = now.Add(time.Duration(p.timers[i].ms) * time.Millisecond)
				due = append(due, p.timers[i])
			}
		}
		p.mu.Unlock()
		for _, tm := range due {
			p.mu.Lock()
			timeout := p.eventTimeout
			if timeout <= 0 {
				timeout = DefaultPluginEventTimeout
			}
			err := runWithTimeout(p.vm, timeout, func() error {
				_, callErr := tm.fn(goja.Undefined(), goja.Undefined())
				return callErr
			})
			p.mu.Unlock()
			if err != nil && p.a != nil && p.a.bus != nil {
				p.a.bus.Publish("plugin:error", map[string]string{
					"plugin": p.name,
					"type":   "timer",
					"error":  err.Error(),
				})
			}
		}
	}
}

// fire runs all handlers registered for an event type.
func (p *pluginVM) fire(eventType string, payload json.RawMessage) {
	p.mu.Lock()
	fns := append([]goja.Callable{}, p.handlers[eventType]...)
	var obj any
	if err := json.Unmarshal(payload, &obj); err != nil {
		obj = string(payload)
	}
	arg := p.vm.ToValue(obj)
	timeout := p.eventTimeout
	if timeout <= 0 {
		timeout = DefaultPluginEventTimeout
	}
	p.mu.Unlock()
	for _, fn := range fns {
		p.mu.Lock()
		err := runWithTimeout(p.vm, timeout, func() error {
			_, callErr := fn(goja.Undefined(), arg)
			return callErr
		})
		p.mu.Unlock()
		if err != nil && p.a != nil && p.a.bus != nil {
			p.a.bus.Publish("plugin:error", map[string]string{
				"plugin": p.name,
				"type":   "event:" + eventType,
				"error":  err.Error(),
			})
		}
	}
}

// Dispatch fans a bus event out to every plugin VM (non-blocking; a slow
// plugin misses ticks, it never stalls the organism).
func (r *JSRuntime) Dispatch(ev bus.Event) {
	r.mu.Lock()
	vms := append([]*pluginVM{}, r.vms...)
	r.mu.Unlock()
	for _, p := range vms {
		select {
		case p.events <- ev:
		default:
		}
	}
}

// PluginNames lists loaded plugins (for the prompt + /api/mind).
func (r *JSRuntime) PluginNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.vms))
	for _, p := range r.vms {
		out = append(out, p.name)
	}
	sort.Strings(out)
	return out
}

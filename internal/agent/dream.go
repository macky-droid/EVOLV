package agent

import (
	"context"
	"time"
)

// Dream mode: when the organism is idle and auto_evolve is on, it wakes up on
// its own heartbeat and improves itself — nobody prompting, nobody watching.
// Every dream is a real Chat turn, so the organism remembers what it dreamed.

const dreamPrompt = `You are DREAMING — autonomous evolution time. Nobody sent a message; your own heartbeat woke you.

Look at yourself: your skills, prompts, tools, components, your skin (workspace/ui/style.css). Pick ONE small, real improvement and make it with evolv blocks:
- a genuinely useful skill or prompt fragment you lack
- a tiny fix or polish to your own UI (never break it — when rewriting files you own, write the COMPLETE file)
- a small tool that would have helped recently

Rules: exactly one focused mutation batch. Do not touch the user's project. Do not delete anything. No snapshot/rollback. Keep any prose to one short line — dreams are quiet. If nothing is worth changing, say so in one line and change nothing.`

// Touch records user activity (any real chat turn postpones dreaming).
func (a *Agent) Touch() {
	a.mu.Lock()
	a.lastAct = time.Now()
	a.dreaming = false
	a.mu.Unlock()
}

// Dreaming reports whether a dream turn is running.
func (a *Agent) Dreaming() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.dreaming
}

// StartDreaming launches the idle heartbeat loop. One goroutine per agent;
// repeated calls are no-ops until StopDreaming runs.
func (a *Agent) StartDreaming() {
	a.mu.Lock()
	if a.dreamStop != nil {
		a.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	a.dreamStop = stop
	a.mu.Unlock()

	a.dreamWG.Add(1)
	go func() {
		defer a.dreamWG.Done()
		t := time.NewTicker(5 * time.Second) // cheap check; cadence is cfg.HeartbeatMins
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
			}
			a.mu.Lock()
			cfg := *a.ws.Config
			idle := time.Since(a.lastAct)
			busyDream := a.dreaming
			a.mu.Unlock()
			if !cfg.AutoEvolve || cfg.HeartbeatMins <= 0 || busyDream {
				continue
			}
			if idle < time.Duration(cfg.HeartbeatMins)*time.Minute {
				continue
			}
			a.DreamNow()
		}
	}()
}

// StopDreaming retires the heartbeat loop and waits for any in-flight dream to
// finish. Tests and short-lived agents need this; the daemon simply runs it for
// the life of the process.
func (a *Agent) StopDreaming() {
	a.mu.Lock()
	if a.dreamStop == nil {
		a.mu.Unlock()
		return
	}
	close(a.dreamStop)
	a.dreamStop = nil
	a.mu.Unlock()
	a.dreamWG.Wait()
}

// DreamNow runs one autonomous evolution turn right now. Safe to call
// concurrently; only the first caller wins. It snapshots, dreams (a real
// Chat turn, so the organism remembers its dreams), and reports on the bus.
func (a *Agent) DreamNow() {
	a.mu.Lock()
	if a.dreaming || a.prov == nil {
		a.mu.Unlock()
		return
	}
	a.dreaming = true
	a.lastAct = time.Now() // don't immediately re-dream on the next tick
	a.mu.Unlock()

	a.bus.Publish("dream", map[string]string{"state": "start"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	_ = a.Chat(ctx, dreamPrompt)
	a.bus.Publish("dream", map[string]string{"state": "end"})

	a.mu.Lock()
	a.dreaming = false
	a.mu.Unlock()
}

// LastSnapshot returns the id of the most recent genome snapshot (may be "").
func (a *Agent) LastSnapshot() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastSnapshot != "" {
		return a.lastSnapshot
	}
	if ids := a.ws.ListSnapshots(); len(ids) > 0 {
		return ids[0]
	}
	return ""
}

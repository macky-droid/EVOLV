// Package execution is EVOLV's execution-mode swarm: a supervisor turns a
// mission into a small task DAG, agents work tasks concurrently, builders
// write real files into an isolated run directory, and testers must record
// real command evidence before the run can pass.
package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dxn111/EVOLV/internal/providers"
)

type Role struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

var Roles = []Role{
	{ID: "architect", Name: "Architect", Icon: "△", Color: "#4f6f8f"},
	{ID: "builder", Name: "Builder", Icon: "▣", Color: "#c96442"},
	{ID: "tester", Name: "Tester", Icon: "✓", Color: "#4f7f68"},
	{ID: "reviewer", Name: "Reviewer", Icon: "◇", Color: "#9a7440"},
	{ID: "integrator", Name: "Integrator", Icon: "⊕", Color: "#7b6390"},
}

func roleByID(id string) Role {
	for _, r := range Roles {
		if r.ID == id {
			return r
		}
	}
	return Role{ID: id, Name: id, Icon: "·", Color: "#8a8174"}
}

type Task struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Role       string     `json:"role"`
	Goal       string     `json:"goal"`
	DependsOn  []string   `json:"depends_on"`
	Acceptance []string   `json:"acceptance"`
	Status     string     `json:"status"` // pending | ready | running | done | failed | blocked
	Agent      string     `json:"agent"`
	Summary    string     `json:"summary,omitempty"`
	Error      string     `json:"error,omitempty"`
	Evidence   []Evidence `json:"evidence,omitempty"`
	Files      []string   `json:"files,omitempty"`
}

type Evidence struct {
	Kind     string `json:"kind"`
	Command  string `json:"command"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Passed   bool   `json:"passed"`
	At       string `json:"at"`
}

type Agent struct {
	ID     string `json:"id"`
	Role   string `json:"role"`
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Color  string `json:"color"`
	Status string `json:"status"` // ready | working | done | failed
	Task   string `json:"task,omitempty"`
	Last   string `json:"last,omitempty"`
}

type Message struct {
	ID    string `json:"id"`
	Agent string `json:"agent"`
	Role  string `json:"role"`
	Kind  string `json:"kind"` // plan | work | evidence | review | final
	Text  string `json:"text"`
	Time  string `json:"time"`
}

type Artifact struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
	Bytes   int    `json:"bytes"`
}

type State struct {
	ID        string     `json:"id"`
	Mission   string     `json:"mission"`
	Status    string     `json:"status"` // idle | running | done | blocked | failed | stopped
	Phase     string     `json:"phase"`
	Provider  string     `json:"provider,omitempty"`
	Model     string     `json:"model,omitempty"`
	Calls     int        `json:"calls,omitempty"`
	ElapsedMS int64      `json:"elapsed_ms,omitempty"`
	Started   string     `json:"started,omitempty"`
	Updated   string     `json:"updated,omitempty"`
	Error     string     `json:"error,omitempty"`
	Verified  bool       `json:"verified"`
	Tasks     []Task     `json:"tasks"`
	Agents    []Agent    `json:"agents"`
	Messages  []Message  `json:"messages"`
	Artifacts []Artifact `json:"artifacts"`
	Root      string     `json:"root,omitempty"`
}

type providerSource func() (providers.Provider, string)
type publisher func(event string, payload any)

type Engine struct {
	mu        sync.Mutex
	source    providerSource
	publish   publisher
	root      string
	state     State
	running   bool
	cancel    context.CancelFunc
	seq       int
	calls     int
	startedAt time.Time

	prov  providers.Provider
	model string
	demo  bool
}

func New(source providerSource, publish publisher, root string) *Engine {
	e := &Engine{source: source, publish: publish, root: root}
	e.state = idleState()
	return e
}

func idleState() State {
	return State{Phase: "idle", Status: "idle", Tasks: []Task{}, Agents: []Agent{}, Messages: []Message{}, Artifacts: []Artifact{}}
}

func (e *Engine) emit() {
	if e.publish != nil {
		e.publish("swarm:exec", e.Snapshot())
	}
}

func (e *Engine) Snapshot() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	return clone(e.state)
}

func clone(s State) State {
	s.Tasks = append([]Task(nil), s.Tasks...)
	for i := range s.Tasks {
		s.Tasks[i].DependsOn = append([]string(nil), s.Tasks[i].DependsOn...)
		s.Tasks[i].Acceptance = append([]string(nil), s.Tasks[i].Acceptance...)
		s.Tasks[i].Evidence = append([]Evidence(nil), s.Tasks[i].Evidence...)
		s.Tasks[i].Files = append([]string(nil), s.Tasks[i].Files...)
	}
	s.Agents = append([]Agent(nil), s.Agents...)
	s.Messages = append([]Message(nil), s.Messages...)
	s.Artifacts = append([]Artifact(nil), s.Artifacts...)
	return s
}

func (e *Engine) Start(ctx context.Context, mission string) (State, error) {
	mission = strings.TrimSpace(mission)
	if mission == "" {
		return State{}, fmt.Errorf("mission is required")
	}
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return State{}, fmt.Errorf("an execution run is already active")
	}
	runID := fmt.Sprintf("exec-%d", time.Now().UnixNano())
	runDir := filepath.Join(e.root, "swarm", "exec", runID)
	if err := os.MkdirAll(filepath.Join(runDir, "project"), 0o755); err != nil {
		e.mu.Unlock()
		return State{}, err
	}
	if err := os.WriteFile(filepath.Join(runDir, "mission.txt"), []byte(mission+"\n"), 0o644); err != nil {
		e.mu.Unlock()
		return State{}, err
	}
	runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	e.state = idleState()
	e.state.ID = runID
	e.state.Mission = mission
	e.state.Root = runDir
	e.state.Status = "running"
	e.state.Phase = "planning"
	e.state.Started = now()
	e.state.Updated = e.state.Started
	e.running = true
	e.cancel = cancel
	e.seq, e.calls = 0, 0
	e.startedAt = time.Now()
	e.mu.Unlock()
	e.emit()
	go func() {
		defer func() {
			e.mu.Lock()
			e.running = false
			e.cancel = nil
			if e.state.Status == "running" {
				e.state.Status = "failed"
				e.state.Phase = "failed"
			}
			e.mu.Unlock()
			e.emit()
			cancel()
		}()
		e.run(runCtx)
	}()
	return e.Snapshot(), nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	cancel := e.cancel
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (e *Engine) run(ctx context.Context) {
	prov, model := (providers.Provider)(nil), ""
	if e.source != nil {
		prov, model = e.source()
	}
	demo := false
	if prov == nil || strings.EqualFold(prov.Name(), "mock") {
		prov = demoProvider{}
		model = "exec-demo"
		demo = true
	}
	providerName := prov.Name()
	if demo {
		providerName = "demo"
	}
	e.mu.Lock()
	e.prov, e.model, e.demo = prov, model, demo
	e.state.Provider, e.state.Model = providerName, model
	e.mu.Unlock()

	tasks, err := e.plan(ctx)
	if err != nil {
		e.fail(ctx, err)
		return
	}
	e.setTasks(tasks)
	e.setPhase("executing")

	for {
		ready := e.readyTasks()
		if len(ready) == 0 {
			break
		}
		if err := e.runBatch(ctx, ready); err != nil {
			e.fail(ctx, err)
			return
		}
		if e.blocked() {
			e.finish(ctx, "blocked", "blocked", "a verification gate failed")
			return
		}
	}

	if !e.verified() {
		e.finish(ctx, "blocked", "unverified", "no passing test evidence was recorded")
		return
	}
	e.setPhase("integrating")
	e.finish(ctx, "done", "done", "")
}

func (e *Engine) plan(ctx context.Context) ([]Task, error) {
	e.setPhase("planning")
	prompt := "Mission: " + e.mission() + `

You are the supervisor of an execution swarm. Decompose the mission into 3-6 concrete tasks with dependencies. Roles: architect, builder, tester, reviewer, integrator. A tester must write files and a verification command. Reply with JSON only:
{"tasks":[{"id":"t1","title":"...","role":"architect","goal":"...","depends_on":[],"acceptance":["..."]}]}`
	text, err := e.complete(ctx, Role{ID: "supervisor", Name: "Supervisor", Icon: "◇"}, prompt)
	if err != nil {
		return nil, err
	}
	tasks := parsePlan(text)
	if len(tasks) == 0 {
		tasks = fallbackPlan()
	}
	return normalizeTasks(tasks), nil
}

func parsePlan(text string) []Task {
	var env struct {
		Tasks []Task `json:"tasks"`
	}
	if decodeJSON(text, &env) == nil && len(env.Tasks) > 0 {
		return env.Tasks
	}
	return nil
}

func fallbackPlan() []Task {
	return []Task{
		{ID: "t1", Title: "Define the build contract", Role: "architect", Goal: "Write the plan, API contract and acceptance checks.", Acceptance: []string{"contract is concrete"}},
		{ID: "t2", Title: "Build the implementation", Role: "builder", Goal: "Implement the mission in real files under the project directory.", DependsOn: []string{"t1"}, Acceptance: []string{"files exist"}},
		{ID: "t3", Title: "Write and run tests", Role: "tester", Goal: "Write a real test suite and a VERIFY command. Execute it and report evidence.", DependsOn: []string{"t2"}, Acceptance: []string{"tests pass"}},
		{ID: "t4", Title: "Review the evidence", Role: "reviewer", Goal: "Audit the implementation and test evidence; name remaining risks.", DependsOn: []string{"t2", "t3"}, Acceptance: []string{"review written"}},
		{ID: "t5", Title: "Integrate the result", Role: "integrator", Goal: "Write the final decision and next step.", DependsOn: []string{"t4"}, Acceptance: []string{"final written"}},
	}
}

func normalizeTasks(in []Task) []Task {
	if len(in) > 8 {
		in = in[:8]
	}
	seen := map[string]bool{}
	for i := range in {
		if in[i].ID == "" {
			in[i].ID = fmt.Sprintf("t%d", i+1)
		}
		in[i].ID = safeID(in[i].ID)
		if seen[in[i].ID] {
			in[i].ID = fmt.Sprintf("%s-%d", in[i].ID, i+1)
		}
		seen[in[i].ID] = true
		if in[i].Role == "" {
			in[i].Role = "builder"
		}
		if in[i].Title == "" {
			in[i].Title = in[i].Goal
		}
		if in[i].Status == "" {
			in[i].Status = "pending"
		}
	}
	// Drop dangling dependencies.
	ids := map[string]bool{}
	for _, t := range in {
		ids[t.ID] = true
	}
	for i := range in {
		deps := []string{}
		for _, dep := range in[i].DependsOn {
			if dep != in[i].ID && ids[dep] {
				deps = append(deps, dep)
			}
		}
		in[i].DependsOn = deps
	}
	return in
}

func (e *Engine) setTasks(tasks []Task) {
	counts := map[string]int{}
	for i := range tasks {
		counts[tasks[i].Role]++
		tasks[i].Agent = fmt.Sprintf("%s-%02d", tasks[i].Role, counts[tasks[i].Role])
	}
	e.mu.Lock()
	e.state.Tasks = tasks
	e.state.Agents = nil
	counts = map[string]int{}
	for _, t := range tasks {
		role := roleByID(t.Role)
		counts[t.Role]++
		e.state.Agents = append(e.state.Agents, Agent{
			ID: t.Agent, Role: t.Role, Name: fmt.Sprintf("%s %02d", role.Name, counts[t.Role]),
			Icon: role.Icon, Color: role.Color, Status: "ready", Task: t.ID,
		})
	}
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) readyTasks() []Task {
	e.mu.Lock()
	defer e.mu.Unlock()
	done := map[string]bool{}
	for _, t := range e.state.Tasks {
		if t.Status == "done" {
			done[t.ID] = true
		}
	}
	out := []Task{}
	for _, t := range e.state.Tasks {
		if t.Status != "pending" {
			continue
		}
		ok := true
		for _, dep := range t.DependsOn {
			if !done[dep] {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, t)
		}
	}
	return out
}

func (e *Engine) runBatch(ctx context.Context, ready []Task) error {
	if len(ready) > 2 {
		ready = ready[:2]
	}
	var wg sync.WaitGroup
	for _, task := range ready {
		if task.Role == "tester" {
			continue // testers run after builders in the same wave
		}
		e.setTaskStatus(task.ID, "running", "")
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			e.runTask(ctx, t)
		}(task)
	}
	wg.Wait()
	// Testers run once the non-tester tasks settle.
	for _, task := range ready {
		if task.Role != "tester" {
			continue
		}
		e.setTaskStatus(task.ID, "running", "")
		e.runTask(ctx, task)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func (e *Engine) runTask(ctx context.Context, task Task) {
	e.setAgentStatus(task.ID, "working", "working")
	contextText := e.taskContext(task)
	user := "Mission: " + e.mission() + "\nTask: " + task.Title + "\nGoal: " + task.Goal + "\nAcceptance:\n"
	for _, a := range task.Acceptance {
		user += "- " + a + "\n"
	}
	user += "\nDependency context:\n" + contextText
	user += "\n\n" + roleInstructions(task.Role)

	text, err := e.complete(ctx, roleByID(task.Role), user)
	if err != nil {
		e.setTaskStatus(task.ID, "failed", err.Error())
		e.setAgentStatus(task.ID, "failed", "failed")
		return
	}
	files := parseFiles(text)
	for _, f := range files {
		path, err := e.writeFile(task.ID, f.Name, f.Content)
		if err != nil {
			e.setTaskStatus(task.ID, "failed", err.Error())
			e.setAgentStatus(task.ID, "failed", "failed")
			return
		}
		e.addArtifact(task, path, "file", f.Content)
		e.addTaskFile(task.ID, path)
	}
	if task.Role == "tester" {
		command := parseVerify(text)
		if command == "" {
			e.addEvidence(task, Evidence{Kind: "test", Command: "", Output: "no VERIFY command was found", ExitCode: -1, Passed: false, At: now()})
			e.setTaskStatus(task.ID, "failed", "tester produced no VERIFY command")
			e.setAgentStatus(task.ID, "failed", "no evidence")
			return
		}
		ev := e.execute(ctx, task, command)
		e.addEvidence(task, ev)
		e.setTaskSummary(task.ID, "VERIFY: "+ev.Command+"\n"+ev.Output)
		if !ev.Passed {
			e.setTaskStatus(task.ID, "failed", "verification failed: "+ev.Command)
			e.setAgentStatus(task.ID, "failed", "tests failed")
			return
		}
	}
	display := summarizeOutput(text, files)
	e.addMessage(task, "work", display)
	e.setTaskSummary(task.ID, display)
	e.setTaskStatus(task.ID, "done", "")
	e.setAgentStatus(task.ID, "done", "done")
}

func (e *Engine) taskContext(task Task) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var b strings.Builder
	for _, dep := range task.DependsOn {
		for _, t := range e.state.Tasks {
			if t.ID != dep {
				continue
			}
			b.WriteString("- " + t.Title + ": " + clip(t.Summary, 700) + "\n")
			for _, ev := range t.Evidence {
				fmt.Fprintf(&b, "  evidence: %s → exit %d (%s)\n", ev.Command, ev.ExitCode, ev.Output)
			}
		}
	}
	return b.String()
}

func (e *Engine) execute(ctx context.Context, task Task, command string) Evidence {
	ev := Evidence{Kind: "test", Command: command, At: now(), ExitCode: -1}
	if !allowedCommand(command) {
		ev.Output = "command rejected by execution policy"
		return ev
	}
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "sh", "-c", command)
	cmd.Dir = filepath.Join(e.root, "swarm", "exec", e.runID(), "project")
	cmd.Env = append(os.Environ(), "CI=1", "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	ev.Output = clip(string(out), 4000)
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ev.ExitCode = ee.ExitCode()
		} else {
			ev.ExitCode = 1
			if ev.Output == "" {
				ev.Output = err.Error()
			}
		}
		return ev
	}
	ev.ExitCode = 0
	ev.Passed = true
	e.setPhase("verifying")
	if e.publish != nil {
		e.publish("swarm:exec:evidence", ev)
	}
	return ev
}

func allowedCommand(command string) bool {
	cmd := strings.TrimSpace(command)
	if strings.ContainsAny(cmd, ";&|`$<>\\\n") {
		return false
	}
	allowed := []string{
		"go test ", "go build ", "go vet ",
		"npm test", "npm run build", "npm run test",
		"pytest", "python -m pytest", "node --test",
		"cargo test", "cargo build",
	}
	for _, prefix := range allowed {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

func (e *Engine) writeFile(taskID, name, content string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") || strings.Contains(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe artifact path %q", name)
	}
	runDir := filepath.Join(e.root, "swarm", "exec", e.runID(), "project")
	full := filepath.Join(runDir, clean)
	if !strings.HasPrefix(full, runDir+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact escapes run directory")
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return "", err
	}
	return full, nil
}

func (e *Engine) runID() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state.ID
}

func (e *Engine) mission() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state.Mission
}

func (e *Engine) complete(ctx context.Context, role Role, user string) (string, error) {
	if e.demo {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(35 * time.Millisecond):
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	e.mu.Lock()
	prov, model := e.prov, e.model
	e.mu.Unlock()
	var b strings.Builder
	last := time.Time{}
	e.activity(role.ID, "thinking…")
	text, err := prov.Stream(callCtx, model, []providers.Message{
		{Role: "system", Content: rolePrompt(role)},
		{Role: "user", Content: user},
	}, func(d providers.Delta) {
		if d.Content == "" {
			return
		}
		b.WriteString(d.Content)
		if now := time.Now(); now.Sub(last) > 120*time.Millisecond {
			last = now
			e.activity(role.ID, clip(b.String(), 160))
		}
	})
	e.mu.Lock()
	e.calls++
	e.state.Calls = e.calls
	e.mu.Unlock()
	e.activity(role.ID, "")
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		text = "(empty response)"
	}
	return text, nil
}

func (e *Engine) activity(roleID, text string) {
	if e.publish != nil {
		e.publish("swarm:exec:activity", map[string]string{"role": roleID, "text": text})
	}
}

func (e *Engine) addMessage(task Task, kind, text string) {
	e.mu.Lock()
	e.seq++
	e.state.Messages = append(e.state.Messages, Message{
		ID: fmt.Sprintf("m%d", e.seq), Agent: task.Agent, Role: task.Role, Kind: kind, Text: text, Time: now(),
	})
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) addArtifact(task Task, path, kind, content string) {
	e.mu.Lock()
	e.state.Artifacts = append(e.state.Artifacts, Artifact{
		Name: filepath.Base(path), Path: path, Kind: kind, Content: clip(content, 6000), Bytes: len(content),
	})
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) addEvidence(task Task, ev Evidence) {
	e.mu.Lock()
	for i := range e.state.Tasks {
		if e.state.Tasks[i].ID == task.ID {
			e.state.Tasks[i].Evidence = append(e.state.Tasks[i].Evidence, ev)
		}
	}
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) addTaskFile(taskID, path string) {
	e.mu.Lock()
	for i := range e.state.Tasks {
		if e.state.Tasks[i].ID == taskID {
			e.state.Tasks[i].Files = append(e.state.Tasks[i].Files, path)
		}
	}
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) setTaskStatus(id, status, errText string) {
	e.mu.Lock()
	for i := range e.state.Tasks {
		if e.state.Tasks[i].ID == id {
			e.state.Tasks[i].Status = status
			if status == "done" && errText == "" {
				e.state.Tasks[i].Summary = "completed with recorded evidence"
			}
			if errText != "" {
				e.state.Tasks[i].Error = errText
			}
		}
	}
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) setTaskSummary(taskID, summary string) {
	e.mu.Lock()
	for i := range e.state.Tasks {
		if e.state.Tasks[i].ID == taskID {
			e.state.Tasks[i].Summary = clip(summary, 1400)
		}
	}
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) setAgentStatus(taskID, status, last string) {
	e.mu.Lock()
	for i := range e.state.Agents {
		if e.state.Agents[i].Task == taskID {
			e.state.Agents[i].Status = status
			e.state.Agents[i].Last = last
		}
	}
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) setPhase(phase string) {
	e.mu.Lock()
	e.state.Phase = phase
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) blocked() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, t := range e.state.Tasks {
		if t.Status == "failed" {
			return true
		}
	}
	return false
}

func (e *Engine) verified() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, t := range e.state.Tasks {
		if t.Role != "tester" {
			continue
		}
		for _, ev := range t.Evidence {
			if ev.Passed {
				return true
			}
		}
	}
	return false
}

func (e *Engine) finish(ctx context.Context, status, phase, errText string) {
	e.mu.Lock()
	e.state.Status = status
	e.state.Phase = phase
	e.state.Error = errText
	e.state.Verified = e.verifiedLocked()
	if !e.startedAt.IsZero() {
		e.state.ElapsedMS = time.Since(e.startedAt).Milliseconds()
	}
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) verifiedLocked() bool {
	for _, t := range e.state.Tasks {
		if t.Role == "tester" {
			for _, ev := range t.Evidence {
				if ev.Passed {
					return true
				}
			}
		}
	}
	return false
}

func (e *Engine) fail(ctx context.Context, err error) {
	status, phase := "failed", "failed"
	if ctx.Err() != nil {
		status, phase = "stopped", "stopped"
	}
	e.finish(ctx, status, phase, err.Error())
}

type fileSpec struct {
	Name    string
	Content string
}

var fileBlock = regexp.MustCompile("(?s)```file:([^\\n`]+)\\n(.*?)```")
var verifyLine = regexp.MustCompile(`(?mi)^VERIFY:\s*(.+)$`)

func parseFiles(text string) []fileSpec {
	matches := fileBlock.FindAllStringSubmatch(text, -1)
	out := make([]fileSpec, 0, len(matches))
	for _, m := range matches {
		out = append(out, fileSpec{Name: strings.TrimSpace(m[1]), Content: strings.TrimSpace(m[2]) + "\n"})
	}
	return out
}

func summarizeOutput(text string, files []fileSpec) string {
	cleaned := strings.TrimSpace(fileBlock.ReplaceAllString(text, ""))
	if len(files) == 0 {
		return text
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name)
	}
	written := "Wrote " + strings.Join(names, ", ") + "."
	if cleaned == "" {
		return written
	}
	return cleaned + "\n\n" + written
}

func parseVerify(text string) string {
	if m := verifyLine.FindStringSubmatch(text); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func rolePrompt(r Role) string {
	return "You are the " + r.Name + " in EVOLV's execution swarm. Real files and real test evidence matter; text alone is not proof. Work only on your assigned task."
}

func roleInstructions(role string) string {
	switch role {
	case "architect":
		return "Write the implementation plan, contracts and acceptance criteria. Be concrete."
	case "builder":
		return "Implement the task in real files. Every file must be a fenced block exactly like:\n```file:relative/path\ncontent\n```\nUse only relative paths inside the project. No prose outside the blocks unless needed."
	case "tester":
		return "Write real test files using fenced file blocks, then add one line:\nVERIFY: <command>\nAllowed commands: go test, go build, go vet, npm test, npm run build, pytest, node --test, cargo test, cargo build. The command must pass."
	case "reviewer":
		return "Audit the implementation and the recorded evidence. Name the strongest part, the biggest risk, and whether the acceptance criteria are met."
	case "integrator":
		return "Write the final decision: what was built, what was verified, what remains unverified, and the next concrete step."
	default:
		return "Do the task concretely."
	}
}

func decodeJSON(text string, v any) error {
	start := strings.IndexByte(text, '{')
	if start < 0 {
		return fmt.Errorf("no json object")
	}
	return json.NewDecoder(strings.NewReader(text[start:])).Decode(v)
}

func safeID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "task"
	}
	return b.String()
}

func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// keep sort imported for deterministic task order in future extensions.
var _ = sort.Slice

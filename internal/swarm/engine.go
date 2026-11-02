// Package swarm runs a small colony of role-specialised agents: one proposes
// directions, one builds, one tests, one critiques and one judges. They share
// a transcript, speak in turns, and settle decisions with polls (one model
// call per voter) instead of a single monolithic answer.
package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dxn111/EVOLV/internal/providers"
)

// Role is a member of the colony.
type Role struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
	Blurb string `json:"blurb"`
}

// Roles is the colony roster. Keep IDs stable: the UI and the prompts use them.
var Roles = []Role{
	{ID: "ideator", Name: "Ideator", Icon: "✦", Color: "#c96442", Blurb: "finds the shape"},
	{ID: "coder", Name: "Coder", Icon: "⌘", Color: "#4f6f8f", Blurb: "builds the thing"},
	{ID: "tester", Name: "Tester", Icon: "✓", Color: "#4f7f68", Blurb: "breaks it first"},
	{ID: "critic", Name: "Critic", Icon: "◇", Color: "#9a7440", Blurb: "argues with everyone"},
	{ID: "judge", Name: "Judge", Icon: "⚖", Color: "#7b6390", Blurb: "calls the vote"},
}

func roleByID(id string) (Role, bool) {
	for _, r := range Roles {
		if r.ID == id {
			return r, true
		}
	}
	return Role{}, false
}

// Message is one line in the shared colony transcript.
type Message struct {
	ID   string `json:"id"`
	Role string `json:"role"`
	Kind string `json:"kind"` // idea | code | test | review | final | system
	Text string `json:"text"`
	Time string `json:"time"`
}

// Vote is one agent's answer to a poll. Choice is zero-based.
type Vote struct {
	Voter  string `json:"voter"`
	Choice int    `json:"choice"`
	Option string `json:"option"`
	Reason string `json:"reason"`
}

// Poll is a decision point. Open polls accept votes; closed polls have a winner.
type Poll struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Votes    []Vote   `json:"votes"`
	Winner   int      `json:"winner"`
	Open     bool     `json:"open"`
}

// AgentState is the roster card in the UI.
type AgentState struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Color  string `json:"color"`
	Blurb  string `json:"blurb"`
	Status string `json:"status"` // ready | thinking | voted | done
	Last   string `json:"last,omitempty"`
}

// Artifact is something the colony produced (code, test plan, final answer).
type Artifact struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
}

// State is the complete UI snapshot. Every change is broadcast as swarm:state.
type State struct {
	ID        string       `json:"id"`
	Mission   string       `json:"mission"`
	Phase     string       `json:"phase"`
	Status    string       `json:"status"` // idle | running | done | stopped | error
	Started   string       `json:"started,omitempty"`
	Updated   string       `json:"updated,omitempty"`
	Error     string       `json:"error,omitempty"`
	Agents    []AgentState `json:"agents"`
	Messages  []Message    `json:"messages"`
	Poll      *Poll        `json:"poll,omitempty"`
	Polls     []Poll       `json:"polls,omitempty"`
	Artifacts []Artifact   `json:"artifacts"`
}

type providerSource func() (providers.Provider, string)
type publisher func(event string, payload any)

// Engine owns one swarm run at a time. Start is non-blocking; the UI follows
// the run through swarm:state bus events and Snapshot for late joiners.
type Engine struct {
	mu      sync.Mutex
	source  providerSource
	publish publisher

	state   State
	running bool
	cancel  context.CancelFunc
	seq     int

	prov  providers.Provider
	model string
	demo  bool
}

// New builds an idle swarm engine. source may return nil; that selects the
// deterministic demo colony so the page is useful with no model configured.
func New(source providerSource, publish publisher) *Engine {
	e := &Engine{source: source, publish: publish}
	e.state = idleState()
	return e
}

func (e *Engine) emit() {
	if e.publish == nil {
		return
	}
	e.publish("swarm:state", e.Snapshot())
}

func idleState() State {
	agents := make([]AgentState, 0, len(Roles))
	for _, r := range Roles {
		agents = append(agents, AgentState{ID: r.ID, Name: r.Name, Icon: r.Icon, Color: r.Color, Blurb: r.Blurb, Status: "ready"})
	}
	return State{Phase: "idle", Status: "idle", Agents: agents, Messages: []Message{}, Artifacts: []Artifact{}}
}

// Snapshot returns a copy safe to serialize while a run is mutating state.
func (e *Engine) Snapshot() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	return cloneState(e.state)
}

func cloneState(s State) State {
	s.Agents = append([]AgentState(nil), s.Agents...)
	s.Messages = append([]Message(nil), s.Messages...)
	s.Artifacts = append([]Artifact(nil), s.Artifacts...)
	if len(s.Polls) > 0 {
		polls := make([]Poll, len(s.Polls))
		for i := range s.Polls {
			polls[i] = s.Polls[i]
			polls[i].Options = append([]string(nil), s.Polls[i].Options...)
			polls[i].Votes = append([]Vote(nil), s.Polls[i].Votes...)
		}
		s.Polls = polls
	}
	if s.Poll != nil {
		p := *s.Poll
		p.Options = append([]string(nil), s.Poll.Options...)
		p.Votes = append([]Vote(nil), s.Poll.Votes...)
		s.Poll = &p
	}
	return s
}

// Start begins a run and returns the initial snapshot.
func (e *Engine) Start(ctx context.Context, mission string) (State, error) {
	mission = strings.TrimSpace(mission)
	if mission == "" {
		return State{}, fmt.Errorf("mission is required")
	}
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return State{}, fmt.Errorf("a swarm is already running")
	}
	runCtx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	e.state = idleState()
	e.state.ID = fmt.Sprintf("swarm-%d", time.Now().UnixNano())
	e.state.Mission = mission
	e.state.Status = "running"
	e.state.Phase = "recruiting"
	e.state.Started = now()
	e.state.Updated = e.state.Started
	e.running = true
	e.cancel = cancel
	e.seq = 0
	e.mu.Unlock()
	e.emit()

	go func() {
		defer func() {
			e.mu.Lock()
			e.running = false
			e.cancel = nil
			// A run that forgets to set a terminal state should not look alive.
			if e.state.Status == "running" {
				e.state.Status = "done"
				e.state.Phase = "done"
			}
			e.state.Updated = now()
			e.mu.Unlock()
			e.emit()
			cancel()
		}()
		e.run(runCtx)
	}()
	return e.Snapshot(), nil
}

// Stop cancels the in-flight run. The run goroutine observes ctx and posts a
// stopped state; the UI does not need a second event.
func (e *Engine) Stop() {
	e.mu.Lock()
	cancel := e.cancel
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (e *Engine) setStatus(status, phase string) {
	e.mu.Lock()
	e.state.Status = status
	e.state.Phase = phase
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

func (e *Engine) setAgent(id, status, last string) {
	e.mu.Lock()
	for i := range e.state.Agents {
		if e.state.Agents[i].ID == id {
			e.state.Agents[i].Status = status
			if last != "" {
				e.state.Agents[i].Last = last
			}
			break
		}
	}
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) addMessage(role, kind, text string) {
	e.mu.Lock()
	e.seq++
	e.state.Messages = append(e.state.Messages, Message{
		ID: fmt.Sprintf("m%d", e.seq), Role: role, Kind: kind, Text: text, Time: now(),
	})
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) addArtifact(name, kind, content string) {
	e.mu.Lock()
	e.state.Artifacts = append(e.state.Artifacts, Artifact{Name: name, Kind: kind, Content: content})
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) setPoll(p *Poll) {
	e.mu.Lock()
	e.state.Poll = p
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) addVote(v Vote) {
	e.mu.Lock()
	if e.state.Poll != nil {
		e.state.Poll.Votes = append(e.state.Poll.Votes, v)
	}
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) closePoll() *Poll {
	e.mu.Lock()
	p := e.state.Poll
	if p != nil {
		counts := make([]int, len(p.Options))
		for _, v := range p.Votes {
			if v.Choice >= 0 && v.Choice < len(counts) {
				counts[v.Choice]++
			}
		}
		best := 0
		for i := 1; i < len(counts); i++ {
			if counts[i] > counts[best] {
				best = i
			}
		}
		p.Winner = best
		p.Open = false
		closed := *p
		closed.Options = append([]string(nil), p.Options...)
		closed.Votes = append([]Vote(nil), p.Votes...)
		e.state.Polls = append(e.state.Polls, closed)
	}
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
	if p == nil {
		return &Poll{}
	}
	cp := *p
	return &cp
}

func (e *Engine) fail(ctx context.Context, err error) {
	e.mu.Lock()
	if ctx.Err() != nil {
		e.state.Status = "stopped"
		e.state.Phase = "stopped"
	} else {
		e.state.Status = "error"
		e.state.Phase = "error"
		e.state.Error = err.Error()
	}
	e.state.Updated = now()
	e.mu.Unlock()
	e.emit()
}

func (e *Engine) run(ctx context.Context) {
	prov, model := (providers.Provider)(nil), ""
	if e.source != nil {
		prov, model = e.source()
	}
	demo := false
	if prov == nil || strings.EqualFold(prov.Name(), "mock") {
		prov = demoProvider{}
		model = "swarm-demo"
		demo = true
	}
	e.mu.Lock()
	e.prov, e.model, e.demo = prov, model, demo
	e.mu.Unlock()

	e.setPhase("ideating")
	ideasText, err := e.call(ctx, "ideator", e.ideaPrompt())
	if err != nil {
		e.fail(ctx, err)
		return
	}
	ideas := parseIdeas(ideasText)
	if len(ideas) == 0 {
		ideas = []idea{{Title: "The obvious first move", Pitch: clip(ideasText, 400)}}
	}
	e.addMessage("ideator", "idea", formatIdeas(ideas))
	options := make([]string, 0, len(ideas))
	for _, idea := range ideas {
		options = append(options, idea.Title)
	}
	e.setPhase("voting")
	poll := &Poll{ID: "direction", Question: "Which direction should the swarm build?", Options: options, Open: true}
	e.setPoll(poll)
	for _, role := range Roles {
		select {
		case <-ctx.Done():
			e.fail(ctx, ctx.Err())
			return
		default:
		}
		vote, err := e.askVote(ctx, role.ID, poll.Question, options)
		if err != nil {
			e.fail(ctx, err)
			return
		}
		e.addVote(vote)
		e.setAgent(role.ID, "voted", "voted for "+vote.Option)
	}
	closed := e.closePoll()
	winner := 0
	if closed.Winner >= 0 && closed.Winner < len(ideas) {
		winner = closed.Winner
	}
	chosen := ideas[winner]

	e.setPhase("coding")
	code, err := e.call(ctx, "coder", e.coderPrompt(chosen))
	if err != nil {
		e.fail(ctx, err)
		return
	}
	e.addMessage("coder", "code", code)
	e.addArtifact(safeName(chosen.Title)+".txt", "code", code)

	e.setPhase("testing")
	tests, err := e.call(ctx, "tester", e.testerPrompt(chosen, code))
	if err != nil {
		e.fail(ctx, err)
		return
	}
	e.addMessage("tester", "test", tests)
	e.addArtifact(safeName(chosen.Title)+".tests.txt", "test", tests)

	e.setPhase("reviewing")
	review, err := e.call(ctx, "critic", e.criticPrompt(chosen, code, tests))
	if err != nil {
		e.fail(ctx, err)
		return
	}
	e.addMessage("critic", "review", review)

	e.setPhase("shipping")
	shipOptions := []string{"Ship it", "Revise once more"}
	e.setPoll(&Poll{ID: "ship", Question: "Ship this direction?", Options: shipOptions, Open: true})
	for _, role := range Roles {
		vote, err := e.askVote(ctx, role.ID, "Should the colony ship this?", shipOptions)
		if err != nil {
			e.fail(ctx, err)
			return
		}
		e.addVote(vote)
		e.setAgent(role.ID, "voted", "voted "+vote.Option)
	}
	ship := e.closePoll()

	e.setPhase("judging")
	decision := "ship"
	if ship.Winner == 1 {
		decision = "revise"
	}
	final, err := e.call(ctx, "judge", e.judgePrompt(chosen, code, tests, review, decision))
	if err != nil {
		e.fail(ctx, err)
		return
	}
	e.addMessage("judge", "final", final)
	e.addArtifact("final.md", "final", final)
	for _, role := range Roles {
		e.setAgent(role.ID, "done", "done")
	}
	e.setStatus("done", "done")
}

func (e *Engine) call(ctx context.Context, roleID, user string) (string, error) {
	role, ok := roleByID(roleID)
	if !ok {
		return "", fmt.Errorf("unknown role %q", roleID)
	}
	e.setAgent(roleID, "thinking", "")
	if e.demo {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(70 * time.Millisecond):
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	e.mu.Lock()
	prov, model := e.prov, e.model
	e.mu.Unlock()
	text, err := prov.Stream(callCtx, model, []providers.Message{
		{Role: "system", Content: rolePrompt(role)},
		{Role: "user", Content: user + "\n\n" + e.transcript()},
	}, func(providers.Delta) {})
	if err != nil {
		e.setAgent(roleID, "ready", "")
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		text = "(the model returned an empty response)"
	}
	e.setAgent(roleID, "ready", clip(text, 90))
	return text, nil
}

func (e *Engine) transcript() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	start := len(e.state.Messages) - 8
	if start < 0 {
		start = 0
	}
	var b strings.Builder
	b.WriteString("Shared transcript (most recent last):\n")
	for _, m := range e.state.Messages[start:] {
		role, _ := roleByID(m.Role)
		b.WriteString(role.Name + ": " + clip(m.Text, 900) + "\n")
	}
	return b.String()
}

func (e *Engine) askVote(ctx context.Context, roleID, question string, options []string) (Vote, error) {
	var b strings.Builder
	b.WriteString("Poll: " + question + "\nOptions:\n")
	for i, option := range options {
		fmt.Fprintf(&b, "%d. %s\n", i+1, option)
	}
	b.WriteString("\nChoose exactly one option. Reply with JSON only: {\"choice\": <number>, \"reason\": \"one short sentence\"}")
	text, err := e.call(ctx, roleID, b.String())
	if err != nil {
		return Vote{}, err
	}
	choice, reason := parseVote(text, options)
	return Vote{Voter: roleID, Choice: choice, Option: options[choice], Reason: reason}, nil
}

func (e *Engine) ideaPrompt() string {
	return "Mission: " + e.mission() + "\n\nPropose three distinct, buildable directions. For each: a short title and a one-sentence pitch. Reply with JSON only: {\"ideas\":[{\"title\":\"...\",\"pitch\":\"...\"}]}."
}

func (e *Engine) coderPrompt(idea idea) string {
	return "Mission: " + e.mission() + "\nChosen direction: " + idea.Title + " — " + idea.Pitch + "\n\nBuild the smallest working implementation. Return the code or precise build steps in one artifact. Be concrete; no preamble."
}

func (e *Engine) testerPrompt(idea idea, code string) string {
	return "Mission: " + e.mission() + "\nChosen direction: " + idea.Title + "\nImplementation:\n" + clip(code, 4000) + "\n\nWrite the tests and edge cases that would prove or break this. Be concrete and adversarial."
}

func (e *Engine) criticPrompt(idea idea, code, tests string) string {
	return "Mission: " + e.mission() + "\nDirection: " + idea.Title + "\nCode:\n" + clip(code, 3000) + "\nTests:\n" + clip(tests, 2500) + "\n\nCritique the work. Name the strongest part, the biggest risk, and one change you would require before shipping."
}

func (e *Engine) judgePrompt(idea idea, code, tests, review, decision string) string {
	return "Mission: " + e.mission() + "\nDirection: " + idea.Title + "\nCode:\n" + clip(code, 2500) + "\nTests:\n" + clip(tests, 2000) + "\nCritique:\n" + clip(review, 2000) + "\nThe colony voted: " + decision + ".\n\nWrite the final answer: what the swarm decided, what it built, how it was verified, and the next concrete step. Be concise. Do not repeat the whole code."
}

func (e *Engine) mission() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state.Mission
}

func rolePrompt(r Role) string {
	switch r.ID {
	case "ideator":
		return "You are the Ideator of a five-agent swarm. You find the shape of the problem and propose three distinct, buildable directions. Prefer sharp, specific ideas over generic ones. Do not write code."
	case "coder":
		return "You are the Coder of a five-agent swarm. You turn the chosen direction into the smallest real implementation. Be concrete, self-contained and honest about tradeoffs."
	case "tester":
		return "You are the Tester of a five-agent swarm. You break what the Coder built. Name concrete tests, edge cases, failure modes and the exact check that would catch them."
	case "critic":
		return "You are the Critic of a five-agent swarm. You are constructive but hard to impress. Name the strongest part, the biggest risk, and one change required before shipping."
	case "judge":
		return "You are the Judge of a five-agent swarm. You synthesize the colony's work into one decision. Be concise, truthful and actionable. Do not invent verification that did not happen."
	default:
		return "You are a member of a five-agent swarm. Be concrete."
	}
}

type idea struct {
	Title string `json:"title"`
	Pitch string `json:"pitch"`
}

type ideaEnvelope struct {
	Ideas []idea `json:"ideas"`
}

func parseIdeas(text string) []idea {
	var env ideaEnvelope
	if decodeJSON(text, &env) == nil && len(env.Ideas) > 0 {
		out := make([]idea, 0, 3)
		for _, i := range env.Ideas {
			i.Title = strings.TrimSpace(i.Title)
			i.Pitch = strings.TrimSpace(i.Pitch)
			if i.Title != "" {
				out = append(out, i)
			}
			if len(out) == 3 {
				break
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	var out []idea
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*•0123456789.) "))
		if line == "" || len(out) == 3 {
			continue
		}
		title, pitch := line, ""
		if a, b, ok := strings.Cut(line, ":"); ok {
			title, pitch = strings.TrimSpace(a), strings.TrimSpace(b)
		} else if a, b, ok := strings.Cut(line, "—"); ok {
			title, pitch = strings.TrimSpace(a), strings.TrimSpace(b)
		}
		if title != "" {
			out = append(out, idea{Title: title, Pitch: pitch})
		}
	}
	return out
}

func parseVote(text string, options []string) (int, string) {
	var env struct {
		Choice any    `json:"choice"`
		Reason string `json:"reason"`
	}
	if decodeJSON(text, &env) == nil && len(options) > 0 {
		choice := 0
		switch v := env.Choice.(type) {
		case float64:
			choice = int(v) - 1
		case string:
			fmt.Sscanf(v, "%d", &choice)
			choice--
		}
		if choice < 0 || choice >= len(options) {
			choice = 0
		}
		return choice, strings.TrimSpace(env.Reason)
	}
	lower := strings.ToLower(text)
	for i, option := range options {
		if strings.Contains(lower, strings.ToLower(option)) {
			return i, clip(text, 180)
		}
	}
	return 0, clip(text, 180)
}

func decodeJSON(text string, v any) error {
	start := strings.IndexByte(text, '{')
	if start < 0 {
		return fmt.Errorf("no json object")
	}
	return json.NewDecoder(strings.NewReader(text[start:])).Decode(v)
}

func formatIdeas(ideas []idea) string {
	var b strings.Builder
	for i, idea := range ideas {
		fmt.Fprintf(&b, "%d. %s", i+1, idea.Title)
		if idea.Pitch != "" {
			b.WriteString(" — " + idea.Pitch)
		}
		if i < len(ideas)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func safeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "swarm"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

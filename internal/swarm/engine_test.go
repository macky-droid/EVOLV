package swarm

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDemoSwarmCompletes(t *testing.T) {
	e := New(nil, nil)
	if _, err := e.Start(context.Background(), "build a tiny focus timer", 5); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if e.Snapshot().Status == "done" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	st := e.Snapshot()
	if st.Status != "done" {
		t.Fatalf("swarm did not finish: status=%s phase=%s error=%q", st.Status, st.Phase, st.Error)
	}
	if len(st.Messages) < 5 {
		t.Fatalf("expected a shared transcript, got %d messages", len(st.Messages))
	}
	if len(st.Polls) < 2 {
		t.Fatalf("expected direction + ship polls, got %d", len(st.Polls))
	}
	for _, p := range st.Polls {
		if len(p.Votes) != len(Roles) {
			t.Fatalf("poll %s has %d votes, want %d", p.ID, len(p.Votes), len(Roles))
		}
	}
	if len(st.Artifacts) < 2 {
		t.Fatalf("expected code + final artifacts, got %d", len(st.Artifacts))
	}
	for _, a := range st.Agents {
		if a.Status != "done" {
			t.Fatalf("agent %s ended in %s", a.ID, a.Status)
		}
	}
}

func TestStartRequiresMission(t *testing.T) {
	e := New(nil, nil)
	if _, err := e.Start(context.Background(), "   ", 5); err == nil {
		t.Fatal("empty mission accepted")
	}
}

func TestParseIdeas(t *testing.T) {
	ideas := parseIdeas(`{"ideas":[{"title":"A","pitch":"one"},{"title":"B","pitch":"two"},{"title":"C","pitch":"three"},{"title":"D","pitch":"four"}]}`)
	if len(ideas) != 4 || ideas[0].Title != "A" || ideas[3].Title != "D" {
		t.Fatalf("ideas = %+v", ideas)
	}
}

func TestParseVote(t *testing.T) {
	options := []string{"Ship it", "Revise"}
	if choice, _ := parseVote(`{"choice":2,"reason":"needs work"}`, options); choice != 1 {
		t.Fatalf("choice = %d", choice)
	}
	if choice, _ := parseVote("I would revise this", options); choice != 1 {
		t.Fatalf("fallback choice = %d", choice)
	}
	choice, reason := parseVote("totally broken", options)
	if choice != 0 || !strings.Contains(reason, "broken") {
		t.Fatalf("last-resort vote = %d %q", choice, reason)
	}
}

func TestHundredAgentSwarm(t *testing.T) {
	e := New(nil, nil)
	st, err := e.Start(context.Background(), "design a hundred-agent review room", 100)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size != 100 || len(st.Agents) != 100 {
		t.Fatalf("roster = size %d / %d agents", st.Size, len(st.Agents))
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if e.Snapshot().Status == "done" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	final := e.Snapshot()
	if final.Status != "done" {
		t.Fatalf("100-agent swarm did not finish: %s %s %s", final.Status, final.Phase, final.Error)
	}
	if len(final.Polls) < 2 {
		t.Fatalf("polls = %d", len(final.Polls))
	}
	for _, p := range final.Polls {
		if len(p.Votes) != 100 {
			t.Fatalf("poll %s has %d votes, want 100", p.ID, len(p.Votes))
		}
	}
	if len(final.Messages) < 20 {
		t.Fatalf("transcript too small for 100 agents: %d", len(final.Messages))
	}
}

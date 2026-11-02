package swarm

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDemoSwarmCompletes(t *testing.T) {
	e := New(nil, nil)
	if _, err := e.Start(context.Background(), "build a tiny focus timer"); err != nil {
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
	if _, err := e.Start(context.Background(), "   "); err == nil {
		t.Fatal("empty mission accepted")
	}
}

func TestParseIdeas(t *testing.T) {
	ideas := parseIdeas(`{"ideas":[{"title":"A","pitch":"one"},{"title":"B","pitch":"two"},{"title":"C","pitch":"three"},{"title":"D","pitch":"four"}]}`)
	if len(ideas) != 3 || ideas[0].Title != "A" || ideas[2].Title != "C" {
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

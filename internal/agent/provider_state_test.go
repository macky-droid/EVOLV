package agent

import (
	"context"
	"strings"
	"testing"
)

// A missing provider is a configuration error, not a panic: the UI should get
// a clear message and the turn should still be persisted.
func TestChatWithoutProviderReportsError(t *testing.T) {
	a := testAgent(t) // no provider
	out := a.Chat(context.Background(), "hello?")
	if !strings.Contains(out, "no provider") {
		t.Fatalf("unexpected reply: %q", out)
	}
	if got := a.HistoryLen(); got != 2 {
		t.Fatalf("history length = %d, want 2", got)
	}
	if got := a.EvolutionCount(); got != 0 {
		t.Fatalf("mutations = %d, want 0", got)
	}
}

func TestProviderModelIsConsistent(t *testing.T) {
	a := testAgent(t)
	name, model := a.ProviderModel()
	if name != "unknown" || model != "test" {
		t.Fatalf("ProviderModel() = %q, %q", name, model)
	}
}

// Concurrent commits must not lose increments now that the counter is atomic.
func TestEvolutionCounterIsAtomic(t *testing.T) {
	a := testAgent(t)
	const n = 200
	done := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		go func() {
			a.evolution.Add(1)
			done <- struct{}{}
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}
	if got := a.EvolutionCount(); got != n {
		t.Fatalf("EvolutionCount() = %d, want %d", got, n)
	}
}

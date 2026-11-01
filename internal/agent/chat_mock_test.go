package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dxn111/EVOLV/internal/bus"
	"github.com/dxn111/EVOLV/internal/providers"
	"github.com/dxn111/EVOLV/internal/workspace"
)

// mockAgent wires an agent to the offline mock mind.
func mockAgent(t *testing.T) (*Agent, *workspace.W) {
	t.Helper()
	providers.ResetMockState() // each scenario starts from the factory demo brain
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	ws, err := workspace.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := providers.Build("mock", "evolv-sim-1", "", nil)
	return New(ws, bus.New(), p, "evolv-sim-1"), ws
}

func TestChatWithMockEvolves(t *testing.T) {
	a, ws := mockAgent(t)
	_ = a.Chat(t.Context(), "evolve a small clock component")
	if a.EvolutionCount() == 0 {
		t.Fatal("no mutations executed")
	}
	if len(ws.ListSnapshots()) == 0 {
		t.Fatal("no auto snapshot taken before the mutation batch")
	}
}

// The organism must wake up, dream (a real Chat turn), and remember it.
func TestDreamNowEvolvesAndRemembers(t *testing.T) {
	a, ws := mockAgent(t)
	before := a.HistoryLen()
	a.DreamNow() // heartbeat is off by default; direct call must still work
	if a.Dreaming() {
		t.Fatal("dreaming flag stuck on")
	}
	if a.HistoryLen() <= before {
		t.Fatal("dream left no trace in memory")
	}
	// the mock dream grows the deep-work skill into the genome
	if _, err := os.Stat(filepath.Join(ws.Root, "workspace/skills/deep-work.md")); err != nil {
		t.Fatal("dream grew no skill:", err)
	}
}

// Heartbeat: with auto_evolve on and a 1-minute cadence, an idle agent dreams.
func TestHeartbeatTriggersDream(t *testing.T) {
	a, _ := mockAgent(t)
	a.ws.Config.AutoEvolve = true
	a.ws.Config.HeartbeatMins = 1
	a.StartDreaming()
	// simulate long idle
	a.mu.Lock()
	a.lastAct = a.lastAct.Add(-2 * time.Minute)
	a.mu.Unlock()
	deadline := time.Now().Add(9 * time.Second)
	for time.Now().Before(deadline) && a.HistoryLen() == 0 {
		time.Sleep(50 * time.Millisecond)
	}
	if a.HistoryLen() == 0 {
		t.Fatal("heartbeat never woke the organism")
	}
}

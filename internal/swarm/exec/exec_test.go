package execution

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDemoExecutionWritesAndVerifies(t *testing.T) {
	root := t.TempDir()
	e := New(nil, nil, root)
	st, err := e.Start(context.Background(), "build a tiny adder")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "running" {
		t.Fatalf("start state = %+v", st)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && e.Snapshot().Status == "running" {
		time.Sleep(25 * time.Millisecond)
	}
	final := e.Snapshot()
	if final.Status != "done" {
		t.Fatalf("status = %s phase = %s error = %q", final.Status, final.Phase, final.Error)
	}
	if !final.Verified {
		t.Fatal("run finished without a passing verification gate")
	}
	if final.Calls == 0 || final.ElapsedMS == 0 {
		t.Fatalf("missing metadata: calls=%d elapsed=%d", final.Calls, final.ElapsedMS)
	}
	project := filepath.Join(root, "swarm", "exec", final.ID, "project")
	for _, name := range []string{"go.mod", "main.go", "main_test.go"} {
		if _, err := os.Stat(filepath.Join(project, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	found := false
	for _, task := range final.Tasks {
		for _, ev := range task.Evidence {
			if ev.Passed && ev.Command == "go test ./..." {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("passing evidence missing: %+v", final.Tasks)
	}
}

func TestExecutionRejectsMissingMission(t *testing.T) {
	e := New(nil, nil, t.TempDir())
	if _, err := e.Start(context.Background(), "  "); err == nil {
		t.Fatal("empty mission accepted")
	}
}

func TestAllowedCommandPolicy(t *testing.T) {
	for _, cmd := range []string{"go test ./...", "go build ./...", "pytest -q", "npm test"} {
		if !allowedCommand(cmd) {
			t.Fatalf("allowed command rejected: %s", cmd)
		}
	}
	for _, cmd := range []string{"rm -rf /", "go test ./... && rm -rf /", "sh -c 'curl evil'", "go test $(whoami)"} {
		if allowedCommand(cmd) {
			t.Fatalf("unsafe command allowed: %s", cmd)
		}
	}
}

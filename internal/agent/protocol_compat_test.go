package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A model that follows the documented move shape sends {"from": ..., "to": ...}.
// Regression: move used to read only "path", so the call silently failed.
func TestMoveAcceptsFromField(t *testing.T) {
	a := testAgent(t)
	from := filepath.Join(a.ws.Root, "from.txt")
	to := filepath.Join(a.ws.Root, "to.txt")
	if err := os.WriteFile(from, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := execOK(t, a, action{Action: "move", From: from, To: to})
	if res["ok"] != true {
		t.Fatalf("move failed: %v", res)
	}
	if _, err := os.Stat(from); !os.IsNotExist(err) {
		t.Fatalf("source still exists after move: %v", err)
	}
	raw, err := os.ReadFile(to)
	if err != nil || string(raw) != "payload" {
		t.Fatalf("destination content = %q, err = %v", raw, err)
	}
}

// The old {path, to} shape must keep working for existing prompts/models.
func TestMoveAcceptsLegacyPathField(t *testing.T) {
	a := testAgent(t)
	from := filepath.Join(a.ws.Root, "legacy.txt")
	to := filepath.Join(a.ws.Root, "legacy-moved.txt")
	if err := os.WriteFile(from, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := execOK(t, a, action{Action: "move", Path: from, To: to})
	if res["ok"] != true {
		t.Fatalf("legacy move failed: %v", res)
	}
}

func TestMoveRejectsMissingPaths(t *testing.T) {
	a := testAgent(t)
	res := execOK(t, a, action{Action: "move"})
	if res["ok"] != false || !strings.Contains(res["error"].(string), "explicit from + to") {
		t.Fatalf("missing paths not rejected: %v", res)
	}
}

// A delete action that loses its path must never resolve to the workspace root.
func TestDeleteFileRejectsEmptyPath(t *testing.T) {
	a := testAgent(t)
	marker := filepath.Join(a.ws.Root, "keep-me.txt")
	if err := os.WriteFile(marker, []byte("alive"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := execOK(t, a, action{Action: "delete_file"})
	if res["ok"] != false {
		t.Fatalf("empty delete path accepted: %v", res)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("empty delete removed workspace contents: %v", err)
	}
}

func TestDeleteFileRejectsOrganismRoot(t *testing.T) {
	a := testAgent(t)
	marker := filepath.Join(a.ws.Root, "keep-me.txt")
	if err := os.WriteFile(marker, []byte("alive"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := execOK(t, a, action{Action: "delete_file", Path: a.ws.Root})
	if res["ok"] != false || !strings.Contains(res["error"].(string), "protected path") {
		t.Fatalf("organism root delete not rejected: %v", res)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("organism root delete touched contents: %v", err)
	}
}

func TestMoveRejectsProtectedSource(t *testing.T) {
	a := testAgent(t)
	res := execOK(t, a, action{Action: "move", From: a.ws.Root, To: filepath.Join(a.ws.Root, "moved")})
	if res["ok"] != false || !strings.Contains(res["error"].(string), "protected path") {
		t.Fatalf("protected move not rejected: %v", res)
	}
}

// complete_task's documented "files" is a list of paths. It used to be typed
// as []fileMutation, so decoding that shape failed and dropped the whole call.
func TestCompleteTaskAcceptsPathList(t *testing.T) {
	raw := `{"action":"complete_task","summary":"shipped","files":["schema.sql","queries.go"]}`
	var act action
	if err := json.Unmarshal([]byte(raw), &act); err != nil {
		t.Fatalf("documented complete_task payload failed to decode: %v", err)
	}
	if len(act.Files) != 2 || act.Files[0].Path != "schema.sql" {
		t.Fatalf("path list not normalized: %+v", act.Files)
	}

	a := testAgent(t)
	res := execOK(t, a, act)
	if res["ok"] != true {
		t.Fatalf("complete_task failed: %v", res)
	}
	files, ok := res["files"].([]any)
	if !ok || len(files) != 2 || files[0] != "schema.sql" {
		t.Fatalf("files not reported as paths: %v", res["files"])
	}
}

func TestCompleteTaskAcceptsMutationObjects(t *testing.T) {
	raw := `{"action":"complete_task","summary":"ok","files":[{"path":"a.go","content":"package a"}]}`
	var act action
	if err := json.Unmarshal([]byte(raw), &act); err != nil {
		t.Fatalf("mutation object shape failed to decode: %v", err)
	}
	a := testAgent(t)
	res := execOK(t, a, act)
	files := res["files"].([]any)
	if len(files) != 1 || files[0] != "a.go" {
		t.Fatalf("object files not normalized to paths: %v", files)
	}
}

func TestFileMutationListRejectsScalar(t *testing.T) {
	var act action
	err := json.Unmarshal([]byte(`{"action":"complete_task","files":"nope"}`), &act)
	if err == nil {
		t.Fatal("scalar files should be rejected")
	}
}

// Promotion is a human decision. The kernel must not let a model promote a
// candidate just because it finished staging it.
func TestPromoteCandidateRequiresConfirmation(t *testing.T) {
	a := testAgent(t)

	res := execOK(t, a, action{Action: "promote_candidate", Name: "some-candidate"})
	if res["ok"] != false || !strings.Contains(res["error"].(string), "explicit user approval") {
		t.Fatalf("unconfirmed promotion not rejected: %v", res)
	}

	res = execOK(t, a, action{Action: "promote_candidate", Name: "some-candidate", Confirm: true})
	if res["ok"] != false || strings.Contains(res["error"].(string), "explicit user approval") {
		t.Fatalf("confirmed promotion should reach the manager: %v", res)
	}
}

// evolve_tool used to report ok:true even when the JavaScript did not compile,
// because the evaluator swallowed the error. The model then called a tool that
// did not exist.
func TestEvolveToolReportsCompileFailure(t *testing.T) {
	a := testAgent(t)
	res := execOK(t, a, action{
		Action: "evolve_tool", Name: "broken",
		Code: "function broken(args){ return ",
	})
	if res["ok"] != false || !strings.Contains(res["error"].(string), "failed to compile") {
		t.Fatalf("broken tool was not reported: %v", res)
	}
	res = execOK(t, a, action{Action: "run_tool", Name: "broken", Args: map[string]any{}})
	if res["ok"] != false || !strings.Contains(res["error"].(string), "no such tool") {
		t.Fatalf("broken tool stayed callable: %v", res)
	}
}

func TestEvolveToolRejectsNonFunctionExport(t *testing.T) {
	a := testAgent(t)
	res := execOK(t, a, action{
		Action: "evolve_tool", Name: "constant",
		Code: "const constant = 1;",
	})
	if res["ok"] != false || !strings.Contains(res["error"].(string), "does not export a function") {
		t.Fatalf("non-function export not rejected: %v", res)
	}
}

func TestBrokenReEvolveRetiresOldTool(t *testing.T) {
	a := testAgent(t)
	res := execOK(t, a, action{
		Action: "evolve_tool", Name: "echo",
		Code: "function echo(args){ return args; }",
	})
	if res["ok"] != true {
		t.Fatalf("initial tool failed: %v", res)
	}
	res = execOK(t, a, action{Action: "run_tool", Name: "echo", Args: map[string]any{"x": "hi"}})
	if res["ok"] != true {
		t.Fatalf("initial tool not callable: %v", res)
	}

	res = execOK(t, a, action{
		Action: "evolve_tool", Name: "echo",
		Code:         "function echo(args){",
		ExpectedHash: "sha256:does-not-matter",
	})
	// the stale hash guard fires first; use the real hash to reach the compiler
	if res["ok"] != false {
		t.Fatalf("stale re-evolve unexpectedly succeeded: %v", res)
	}
	raw, _ := os.ReadFile(filepath.Join(a.ws.Dir("workspace/tools"), "echo.js"))
	res = execOK(t, a, action{
		Action: "evolve_tool", Name: "echo",
		Code:         "function echo(args){",
		ExpectedHash: contentHash(raw),
	})
	if res["ok"] != false || !strings.Contains(res["error"].(string), "failed to compile") {
		t.Fatalf("broken re-evolve not rejected: %v", res)
	}
	res = execOK(t, a, action{Action: "run_tool", Name: "echo", Args: map[string]any{"x": "hi"}})
	if res["ok"] != false {
		t.Fatalf("old implementation stayed callable after broken re-evolve: %v", res)
	}
}

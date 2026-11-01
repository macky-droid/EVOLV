package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

// mkTestW builds a workspace in a temp home, like the agent tests do.
func mkTestW(t *testing.T) *W {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	w, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestDiffSnapshots(t *testing.T) {
	ws := mkTestW(t)

	// snapshot A: two files
	os.MkdirAll(filepath.Join(ws.Dir("workspace/ui")), 0o755)
	os.WriteFile(filepath.Join(ws.Dir("workspace/ui/style.css")), []byte("a\nb\nc\n"), 0o644)
	os.WriteFile(filepath.Join(ws.Dir("workspace/evolution.jsonl")), []byte("{\"n\":1}\n"), 0o644)
	idA, err := ws.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	// snapshot B: modify one, add one, delete one
	os.WriteFile(filepath.Join(ws.Dir("workspace/ui/style.css")), []byte("a\nX\nc\nd\n"), 0o644)
	os.WriteFile(filepath.Join(ws.Dir("workspace/ui/new.html")), []byte("<p>hi</p>\n"), 0o644)
	os.Remove(filepath.Join(ws.Dir("workspace/evolution.jsonl")))
	idB, err := ws.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	diffs, err := ws.DiffSnapshots(idA, idB)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]FileDiff{}
	for _, d := range diffs {
		byPath[d.Path] = d
	}

	mod, ok := byPath["workspace/ui/style.css"]
	if !ok || mod.Status != "modified" {
		t.Fatalf("style.css should be modified, got %+v", mod)
	}
	added, has := byPath["workspace/ui/new.html"]
	if !has || added.Status != "added" {
		t.Fatalf("new.html should be added, got %+v", added)
	}
	removed, has := byPath["workspace/evolution.jsonl"]
	if !has || removed.Status != "removed" {
		t.Fatalf("evolution.jsonl should be removed, got %+v", removed)
	}

	// check the LCS found the single-line change
	var plus, minus int
	for _, l := range mod.Lines {
		switch l.Tag {
		case "+":
			plus++
		case "-":
			minus--
		}
	}
	if plus == 0 || minus == 0 {
		t.Fatalf("expected + and - lines in modified diff, got %+v", mod.Lines)
	}

	// identical snapshots → empty diff
	empty, err := ws.DiffSnapshots(idA, idA)
	if err != nil || len(empty) != 0 {
		t.Fatalf("self-diff should be empty: %v %v", empty, err)
	}

	// bad ids rejected
	if _, err := ws.DiffSnapshots("../..", idB); err == nil {
		t.Fatal("expected error for bad snapshot id")
	}
}

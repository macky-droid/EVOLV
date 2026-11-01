package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dxn111/EVOLV/internal/workspace"
)

func TestParseSkillMetaFrontmatter(t *testing.T) {
	raw := []byte("---\nname: systematic-debugging\ndescription: Use when a test fails and the cause is unclear.\n---\n# Systematic Debugging\n\nReproduce first.\n")
	meta := parseSkillMeta("whatever.md", raw)
	if meta.Name != "systematic-debugging" {
		t.Fatalf("name = %q", meta.Name)
	}
	if meta.Title != "Systematic Debugging" {
		t.Fatalf("title = %q", meta.Title)
	}
	if !strings.Contains(meta.Summary, "Use when a test fails") {
		t.Fatalf("summary = %q", meta.Summary)
	}
	if strings.Contains(meta.Summary, "---") {
		t.Fatalf("frontmatter leaked into summary: %q", meta.Summary)
	}
}

func TestParseSkillMetaFallback(t *testing.T) {
	raw := []byte("# Repo Cartography\n\nMap the codebase before reading every file.\n\nMore detail.\n")
	meta := parseSkillMeta("repo-cartography.md", raw)
	if meta.Title != "Repo Cartography" {
		t.Fatalf("title = %q", meta.Title)
	}
	if meta.Summary != "Map the codebase before reading every file." {
		t.Fatalf("summary = %q", meta.Summary)
	}
}

func TestParseSkillMetaHandlesEmptyAndUnterminatedFrontmatter(t *testing.T) {
	meta := parseSkillMeta("bare-name.md", nil)
	if meta.Title != "Bare Name" || meta.Summary != "" {
		t.Fatalf("empty skill parsed as %+v", meta)
	}
	meta = parseSkillMeta("broken.md", []byte("---\nname: x\n# No closing delimiter\nbody"))
	if meta.Name != "broken" {
		t.Fatalf("unterminated frontmatter swallowed the name: %+v", meta)
	}
}

func TestSkillIndexInSystemPrompt(t *testing.T) {
	a := testAgent(t)
	writeSkill := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(a.ws.Dir(workspace.SkillsDir), name+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeSkill("debugging", "---\nname: debugging\ndescription: Use when something is broken.\n---\n# Debugging\n\nSECRET-BODY-TEXT\n")
	writeSkill("shipping", "# Shipping\n\nUse when releasing.\n")

	sp := a.SystemPrompt()
	if !strings.Contains(sp, "## SKILLS") {
		t.Fatal("skills section missing from prompt")
	}
	if !strings.Contains(sp, "debugging") || !strings.Contains(sp, "Use when something is broken.") {
		t.Fatalf("skill trigger missing from prompt:\n%s", sp)
	}
	if !strings.Contains(sp, "skills/debugging.md") {
		t.Fatal("skill path missing from prompt")
	}
	if strings.Contains(sp, "SECRET-BODY-TEXT") {
		t.Fatal("full skill body must not be inlined into the prompt")
	}
}

func TestProfileFiltersSkillIndex(t *testing.T) {
	base := testAgent(t)
	for _, name := range []string{"alpha", "beta"} {
		if err := os.WriteFile(filepath.Join(base.ws.Dir(workspace.SkillsDir), name+".md"),
			[]byte("# "+name+"\n\n"+name+" trigger.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	strand := base.CloneProfile(Profile{ID: "strand", Name: "Strand", Skills: []string{"alpha"}})
	sp := strand.SystemPrompt()
	if !strings.Contains(sp, "alpha") {
		t.Fatalf("selected skill missing from strand prompt:\n%s", sp)
	}
	if strings.Contains(sp, "beta") {
		t.Fatalf("unselected skill leaked into strand prompt:\n%s", sp)
	}
}

func TestSystemPromptPlaceholdersAreResolved(t *testing.T) {
	a := testAgent(t)
	sp := a.SystemPrompt()
	if strings.Contains(sp, "{{") {
		t.Fatalf("unresolved placeholder in system prompt:\n%s", sp)
	}
	for _, want := range []string{"HOW YOU WORK", "THE EVOLV PROTOCOL", "SAFETY RAILS", "DEFINITION OF DONE", "stage_candidate"} {
		if !strings.Contains(sp, want) {
			t.Fatalf("prompt is missing %q", want)
		}
	}
}

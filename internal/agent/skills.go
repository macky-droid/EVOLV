package agent

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skills are plain markdown playbooks in workspace/skills/. The prompt gets a
// compact index (trigger + path), not the whole file: a skill is loaded only
// when its trigger matches, which keeps every turn small and makes the skill
// body a deliberate, readable action instead of ambient noise.
//
// A skill may start with YAML frontmatter:
//
//	---
//	name: systematic-debugging
//	description: Use when a build or test fails and the cause is not obvious.
//	---
//	# Systematic Debugging
//	...
//
// Frontmatter is optional. Without it the first H1 becomes the title and the
// first body paragraph becomes the trigger, so hand-written skills still work.

const (
	skillSummaryRunes = 180
	// skillIndexLimit bounds the number of entries injected into the prompt.
	// The agent can still list_dir the rest; this just keeps the header small
	// even for an organism that has grown dozens of skills.
	skillIndexLimit = 40
)

type skillMeta struct {
	Name    string
	Title   string
	Summary string
	Path    string
}

// parseSkillMeta reads a skill's identity without executing or interpreting it.
func parseSkillMeta(fileName string, raw []byte) skillMeta {
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	meta := skillMeta{Name: name, Title: humanizeName(name)}
	body := strings.TrimSpace(string(raw))

	if front, rest, ok := splitFrontmatter(body); ok {
		for _, line := range strings.Split(front, "\n") {
			key, value, found := strings.Cut(line, ":")
			if !found {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "name":
				if v := strings.Trim(strings.TrimSpace(value), `"'`); v != "" {
					meta.Name = v
				}
			case "title":
				if v := strings.Trim(strings.TrimSpace(value), `"'`); v != "" {
					meta.Title = v
				}
			case "description", "summary", "when":
				if v := cleanSkillLine(value); v != "" {
					meta.Summary = v
				}
			}
		}
		body = rest
	}

	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			if title := strings.TrimSpace(strings.TrimPrefix(line, "# ")); title != "" {
				meta.Title = title
			}
			break
		}
	}
	if meta.Summary == "" {
		meta.Summary = firstSkillParagraph(body)
	}
	meta.Summary = clipRunes(meta.Summary, skillSummaryRunes)
	return meta
}

// splitFrontmatter separates a leading YAML block from the markdown body.
func splitFrontmatter(s string) (front, body string, ok bool) {
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return "", s, false
	}
	rest := s[strings.IndexByte(s, '\n')+1:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", s, false
	}
	body = rest[end+4:]
	return rest[:end], strings.TrimSpace(body), true
}

// firstSkillParagraph returns the first non-heading, non-list line of the body.
func firstSkillParagraph(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") ||
			strings.HasPrefix(line, "*") || strings.HasPrefix(line, ">") {
			continue
		}
		return cleanSkillLine(line)
	}
	return ""
}

// cleanSkillLine collapses whitespace and strips wrapping quotes.
func cleanSkillLine(s string) string {
	s = strings.Trim(strings.TrimSpace(s), `"'`)
	return strings.Join(strings.Fields(s), " ")
}

// loadSkillIndex parses every .md file in dir, sorted by name.
func loadSkillIndex(dir string) []skillMeta {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]skillMeta, 0, len(ents))
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		meta := parseSkillMeta(e.Name(), raw)
		meta.Path = "skills/" + e.Name()
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// humanizeName turns "systematic-debugging" into "Systematic Debugging".
func humanizeName(name string) string {
	words := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(name))
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	if len(words) == 0 {
		return name
	}
	return strings.Join(words, " ")
}

// clipRunes truncates s to max runes, adding an ellipsis when it cuts.
func clipRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max-1])) + "…"
}

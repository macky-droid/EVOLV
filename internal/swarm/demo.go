package swarm

import (
	"context"
	"strings"

	"github.com/dxn111/EVOLV/internal/providers"
)

// demoProvider lets the swarm page work with the offline mock provider (or no
// provider at all). It answers role-shaped prompts deterministically so the
// UI, polls and transcript can be exercised before a real model is configured.
type demoProvider struct{}

func (demoProvider) Name() string { return "mock" }

func (demoProvider) Models(context.Context) ([]string, error) {
	return []string{"swarm-demo"}, nil
}

func (demoProvider) Stream(_ context.Context, _ string, msgs []providers.Message, _ func(providers.Delta)) (string, error) {
	system, user := "", ""
	for _, m := range msgs {
		if m.Role == "system" {
			system = m.Content
		}
		if m.Role == "user" {
			user = m.Content
		}
	}
	if strings.Contains(strings.ToLower(user), "reply with json only") && strings.Contains(strings.ToLower(user), "choice") {
		return `{"choice":1,"reason":"the first direction is the sharpest test of the mission"}`, nil
	}
	switch {
	case strings.Contains(system, "Ideator"):
		return `{"ideas":[{"title":"Swarm cockpit","pitch":"a live room where specialised agents argue, build and vote in public"},{"title":"Parallel patch lab","pitch":"three coders explore independent patches against the same frozen tests"},{"title":"Colony memory","pitch":"a shared decision ledger so every vote and artifact is replayable"}]}`, nil
	case strings.Contains(system, "Coder"):
		return "Implementation sketch:\n\n```html\n<section class=\"swarm-cockpit\">\n  <ol id=\"roster\"></ol>\n  <main id=\"thread\"></main>\n  <aside id=\"polls\"></aside>\n</section>\n```\n\nWire the roster to `swarm:state` events, render only the latest state, and keep the transcript append-only.", nil
	case strings.Contains(system, "Tester"):
		return "Tests I would run:\n1. Start with an empty mission → 400, no run created.\n2. Start twice → second call returns 409 while the first is live.\n3. Every poll has one vote per role, and the winner is deterministic on a tie.\n4. Kill the run mid-phase → state becomes stopped, not done.\n5. Reconnect the page → `/api/swarm` returns the same transcript.", nil
	case strings.Contains(system, "Critic"):
		return "Strongest part: the transcript and polls make the reasoning auditable. Biggest risk: a confident coder can still produce code nobody executed. Required change: label artifacts as proposals until a tester actually runs them, and show that distinction in the UI.", nil
	case strings.Contains(system, "Judge"):
		return "The colony chose the Swarm cockpit: a live five-agent room with a shared transcript and explicit polls. It is a proposal, not a verified build. Next step: run the tester's checklist against the real engine and only then treat the artifacts as shippable.", nil
	default:
		return "The demo colony is thinking in role-shaped turns.", nil
	}
}

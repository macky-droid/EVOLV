package execution

import (
	"context"
	"strings"

	"github.com/dxn111/EVOLV/internal/providers"
)

type demoProvider struct{}

func (demoProvider) Name() string { return "mock" }
func (demoProvider) Models(context.Context) ([]string, error) {
	return []string{"exec-demo"}, nil
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
	if strings.Contains(user, "Decompose the mission") {
		return `{"tasks":[
{"id":"contract","title":"Define the build contract","role":"architect","goal":"Write the implementation contract and acceptance checks.","depends_on":[],"acceptance":["contract is concrete"]},
{"id":"build","title":"Build the smallest working implementation","role":"builder","goal":"Implement a tiny Go module that satisfies the contract.","depends_on":["contract"],"acceptance":["files exist"]},
{"id":"test","title":"Write and execute tests","role":"tester","goal":"Write real tests and run them.","depends_on":["build"],"acceptance":["go test passes"]},
{"id":"review","title":"Review the evidence","role":"reviewer","goal":"Audit the build and evidence.","depends_on":["build","test"],"acceptance":["review written"]},
{"id":"final","title":"Integrate the result","role":"integrator","goal":"Write the final decision.","depends_on":["review"],"acceptance":["final written"]}
]}`, nil
	}
	switch {
	case strings.Contains(system, "Architect"):
		return "Contract: a tiny Go module with Add(a,b) returning the sum, covered by one table test. Acceptance: go test ./... exits zero.", nil
	case strings.Contains(system, "Builder"):
		return "```file:go.mod\nmodule genesis-demo\n\ngo 1.21\n```\n```file:main.go\npackage main\n\nfunc Add(a, b int) int { return a + b }\n\nfunc main() {}\n```\n", nil
	case strings.Contains(system, "Tester"):
		return "```file:main_test.go\npackage main\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif got := Add(2, 3); got != 5 {\n\t\tt.Fatalf(\"Add(2,3) = %d\", got)\n\t}\n}\n```\nVERIFY: go test ./...", nil
	case strings.Contains(system, "Reviewer"):
		return "The build is small and the test exercises the contract. Risk: only one case is covered. Acceptance is met for the stated contract.", nil
	case strings.Contains(system, "Integrator"):
		return "Genesis built a tiny Go module, wrote a real test, and recorded a passing go test evidence line. Next: add the missing edge cases before calling it production-ready.", nil
	default:
		return "The execution swarm is working.", nil
	}
}

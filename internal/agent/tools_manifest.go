package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dxn111/EVOLV/internal/workspace"
)

// ---------- typed tool manifests ----------
//
// A grown tool can declare its input contract: {param: type}. The manifest
// (workspace/tools/<name>.json) records name, version and schema so
// run_tool can validate arguments before the code ever runs — bad calls
// fail with a precise type error instead of a JS stack trace.

// toolManifest is the persisted contract of one grown tool.
type toolManifest struct {
	Name        string            `json:"name"`
	Version     int               `json:"version"`
	Description string            `json:"description,omitempty"`
	Schema      map[string]string `json:"schema,omitempty"`
	Updated     string            `json:"updated"`
}

// manifestPath is where a tool's manifest lives.
func manifestPath(a *Agent, name string) string {
	return filepath.Join(a.ws.Dir(workspace.ToolsDir), name+".json")
}

// readToolManifest loads a tool's manifest, or nil when the tool has none
// (older tools stay callable — manifests are opt-in typed contracts).
func readToolManifest(a *Agent, name string) *toolManifest {
	raw, err := os.ReadFile(manifestPath(a, name))
	if err != nil {
		return nil
	}
	var m toolManifest
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return &m
}

// writeToolManifest persists the manifest atomically, bumping version when
// the tool already has one.
func writeToolManifest(a *Agent, m *toolManifest) (int, error) {
	if prev := readToolManifest(a, m.Name); prev != nil {
		m.Version = prev.Version + 1
	} else {
		m.Version = 1
	}
	m.Updated = time.Now().UTC().Format(time.RFC3339)
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return m.Version, err
	}
	tmp := manifestPath(a, m.Name) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return m.Version, err
	}
	return m.Version, os.Rename(tmp, manifestPath(a, m.Name))
}

// validSchemaTypes is the set of argument types a manifest may declare.
var validSchemaTypes = map[string]bool{
	"string": true, "number": true, "boolean": true,
	"object": true, "array": true, "any": true,
}

// validateToolArgs checks args against the manifest schema. Declared params
// must be present with the right Go type; extra args are passed through.
// Parameters are checked in name order so the reported error is deterministic
// (map iteration order previously made the same bad call name "a" or "b" at
// random, which also made the regression test flaky).
func validateToolArgs(schema map[string]string, args map[string]any) error {
	params := make([]string, 0, len(schema))
	for param := range schema {
		params = append(params, param)
	}
	sort.Strings(params)

	for _, param := range params {
		want := strings.ToLower(strings.TrimSpace(schema[param]))
		if !validSchemaTypes[want] {
			return fmt.Errorf("tool schema declares unknown type %q for %q (use string|number|boolean|object|array|any)", want, param)
		}
		v, ok := args[param]
		if !ok || v == nil {
			return fmt.Errorf("missing required arg %q (%s)", param, want)
		}
		if want == "any" {
			continue
		}
		var pass bool
		switch want {
		case "string":
			_, pass = v.(string)
		case "number":
			_, pass = v.(float64) // JSON numbers decode to float64
		case "boolean":
			_, pass = v.(bool)
		case "object":
			_, pass = v.(map[string]any)
		case "array":
			_, pass = v.([]any)
		}
		if !pass {
			return fmt.Errorf("arg %q must be %s, got %T", param, want, v)
		}
	}
	return nil
}

// pluginPermsFromManifest reads a plugin's persisted perms from its
// plugin.json so reload-time sandboxes get the same capabilities the
// author asked for at install time.
func pluginPermsFromManifest(a *Agent, name string) []string {
	raw, err := os.ReadFile(filepath.Join(a.ws.Dir(workspace.PluginsDir), name, "plugin.json"))
	if err != nil {
		return nil
	}
	var m struct {
		Perms []string `json:"perms"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m.Perms
}

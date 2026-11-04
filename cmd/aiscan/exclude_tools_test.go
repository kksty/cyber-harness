package main

import (
	"context"
	"testing"

	agentsession "github.com/chainreactors/cyber/agent/session"
	ioatools "github.com/chainreactors/cyber/tools/ioa"
)

// loadAIScan builds, loads and returns one profile plus its session runtime.
func loadAIScan(t *testing.T, c config) (*aiscanProfile, *agentsession.Runtime) {
	t.Helper()
	profile, err := buildAIScanProfile(c)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	if err := profile.Load(t.Context()); err != nil {
		t.Fatalf("load: %v", err)
	}
	t.Cleanup(func() { _ = profile.Close(context.Background()) })
	runtime, err := profile.Runtime()
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	return profile, runtime
}

// hasTool reports whether the runtime's model-facing tool set contains name.
func hasTool(runtime *agentsession.Runtime, name string) bool {
	tools := runtime.Tools()
	if tools == nil {
		return false
	}
	for _, definition := range tools.ToolDefinitions() {
		if definition.Name == name {
			return true
		}
	}
	return false
}

// The environment list is case-insensitive, trims whitespace, ignores blank
// entries and treats unset/empty as "exclude nothing".
func TestExcludedToolsParsing(t *testing.T) {
	for _, test := range []struct {
		value string
		want  []string
	}{
		{"", nil},
		{" , ,, ", nil},
		{"subagent", []string{"subagent"}},
		{" SubAgent , IOA ", []string{"ioa", "subagent"}},
		{"ioa", []string{"ioa"}},
	} {
		t.Run(test.value, func(t *testing.T) {
			t.Setenv(excludeToolsEnv, test.value)
			got := excludedToolList()
			if len(got) != len(test.want) {
				t.Fatalf("excluded = %v, want %v", got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("excluded = %v, want %v", got, test.want)
				}
			}
		})
	}
}

// A session profile composes the native subagent tool by default; excluding it
// must remove the model-facing tool without breaking the composition.
func TestExcludedSubagentRemovesSessionTool(t *testing.T) {
	t.Setenv(excludeToolsEnv, "")
	_, defRuntime := loadAIScan(t, minimalConfig(&agentsession.Config{}))
	if !hasTool(defRuntime, "subagent") {
		t.Fatal("the default profile must compose the native subagent tool")
	}

	t.Setenv(excludeToolsEnv, "subagent")
	_, excludedRuntime := loadAIScan(t, minimalConfig(&agentsession.Config{}))
	if hasTool(excludedRuntime, "subagent") {
		t.Fatal("CYBER_EXCLUDE_TOOLS=subagent did not remove the native subagent tool")
	}
}

// Engine extensions borrow the Subagent capability for scanner definitions. The
// exclusion removes the model tool, not the capability, so a profile with engines
// still composes and loads.
func TestExcludedSubagentKeepsEngineComposition(t *testing.T) {
	t.Setenv(excludeToolsEnv, "subagent")
	c := minimalConfig(&agentsession.Config{})
	c.Base.SkipEngines = false
	_, runtime := loadAIScan(t, c)
	if hasTool(runtime, "subagent") {
		t.Fatal("engine composition re-exposed the subagent tool")
	}
}

// Excluding "ioa" drops the agent-visible IOA surface (commands/prompt/console),
// not an IOA server the host runs. The profile must compose and load, and the
// model must no longer see the `ioa` command.
func TestExcludedIOAOmitsAgentSurface(t *testing.T) {
	t.Setenv(excludeToolsEnv, "ioa")
	c := minimalConfig(&agentsession.Config{})
	c.IOA = &ioatools.Config{NodeName: "test-node"}
	_, runtime := loadAIScan(t, c)
	if registry := runtime.CommandRegistry(); registry != nil && registry.Has("ioa") {
		t.Fatal("CYBER_EXCLUDE_TOOLS=ioa still registered the ioa command")
	}
}

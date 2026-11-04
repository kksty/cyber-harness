package main

import (
	"os"
	"sort"
	"strings"
)

// CYBER_EXCLUDE_TOOLS is the deployment's "do not expose these agent tools"
// list. It exists because some embedded hosts must keep the ledger as the only
// source of delegation: a model that can call the harness's own `subagent` tool
// starts work the host cannot see, cancel, or account for (tianwu P0-5). The
// capability itself stays installed where other extensions need it; only the
// model-facing tool is removed.
//
// The value is a comma-separated, case-insensitive list of tool names. Empty or
// unset means "exclude nothing", which is the historical behaviour: no host that
// has not asked for this gets a different tool surface.
//
// Recognised names: "subagent" (the native delegation tool) and "ioa" (the IOA
// collaboration surface: client commands, prompt section and console bindings).
// "ioa" never removes an IOA server: hosts that run one (the Web host) keep their
// message transport.
const excludeToolsEnv = "CYBER_EXCLUDE_TOOLS"

// excludedTools parses CYBER_EXCLUDE_TOOLS into a lower-cased set. It reads the
// environment on every call so tests can t.Setenv per case and a long-lived
// process is not stuck with the first composition's decision.
func excludedTools() map[string]bool {
	raw := os.Getenv(excludeToolsEnv)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	out := make(map[string]bool)
	for _, name := range strings.Split(raw, ",") {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			out[name] = true
		}
	}
	return out
}

// toolExcluded reports whether one tool name was excluded.
func toolExcluded(name string) bool {
	return excludedTools()[strings.ToLower(strings.TrimSpace(name))]
}

// excludedToolList is the sorted form used by the startup log, so the operator
// can see in the host's stderr what the process actually removed.
func excludedToolList() []string {
	set := excludedTools()
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

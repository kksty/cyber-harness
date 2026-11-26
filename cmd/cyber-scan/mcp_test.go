package main

import (
	"testing"

	"github.com/chainreactors/cyber/agent"
	agentsession "github.com/chainreactors/cyber/agent/session"
	guardrailext "github.com/chainreactors/cyber/exts/guardrail"
	mcpext "github.com/chainreactors/cyber/exts/mcp"
	mcptools "github.com/chainreactors/cyber/tools/mcp"
)

// External MCP servers are operator configuration: the reference graph must be
// identical to a build without the extension unless the operator configured at
// least one server.
func TestExtensionsInstallMCPOnlyWhenServersAreConfigured(t *testing.T) {
	for _, test := range []struct {
		name    string
		servers map[string]mcptools.Config
		want    bool
	}{
		{name: "section absent"},
		{name: "section empty", servers: map[string]mcptools.Config{}},
		{
			name:    "servers configured",
			servers: map[string]mcptools.Config{"ida": {Command: "ida-mcp"}},
			want:    true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			application := minimalConfig(&agentsession.Config{}).Base
			application.DataDir = t.TempDir()
			application.MCP = mcpext.Config{MCPServers: test.servers}
			values, err := extensions(application, agent.NoLoop(), t.TempDir(), nil, guardrailext.Config{})
			if err != nil {
				t.Fatal(err)
			}
			installed := false
			for _, value := range values {
				if _, ok := value.(*mcpext.Extension); ok {
					installed = true
				}
			}
			if installed != test.want {
				t.Fatalf("MCP extension installed = %v, want %v", installed, test.want)
			}
		})
	}
}

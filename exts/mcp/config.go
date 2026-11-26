package mcp

import (
	"github.com/chainreactors/cyber/core/resource"
	cfg "github.com/chainreactors/cyber/pkg/config"
	mcptools "github.com/chainreactors/cyber/tools/mcp"
)

// ConfigKey is the configuration section owned by this extension.
const ConfigKey = "mcp"

// Config is the section body: one named map of MCP servers. The section stays
// inert until a server is configured, so an omitted `mcp` block keeps the
// extension out of the composition.
type Config struct {
	MCPServers map[string]mcptools.Config `json:"mcpServers"`
}

// Section declares the MCP configuration section. An omitted section keeps the
// extension out of the composition; configured servers are installed as native
// bash commands.
func Section() cfg.Section {
	return cfg.Section{
		Key: ConfigKey,
		New: func() any { return &Config{MCPServers: map[string]mcptools.Config{}} },
	}
}

func Declare(resources *resource.Registry) error {
	_, err := resource.Add[cfg.Section](resources, Section())
	return err
}

// ReadConfig decodes the MCP section from a parsed option set. The returned
// config owns its server map; callers never share the resolved snapshot.
func ReadConfig(option *cfg.Option) (Config, error) {
	var value *Config
	if option == nil {
		return Config{}, nil
	}
	if option.Resolved != nil {
		resolved, err := cfg.Get[*Config](option.Resolved, ConfigKey)
		if err != nil {
			return Config{}, err
		}
		value = resolved
	} else {
		raw, err := Section().Decode(option.Extensions[ConfigKey])
		if err != nil {
			return Config{}, err
		}
		value = raw.(*Config)
	}
	return Config{MCPServers: cloneServers(value.MCPServers)}, nil
}

func cloneServers(servers map[string]mcptools.Config) map[string]mcptools.Config {
	if servers == nil {
		return nil
	}
	out := make(map[string]mcptools.Config, len(servers))
	for name, server := range servers {
		out[name] = server.Clone()
	}
	return out
}

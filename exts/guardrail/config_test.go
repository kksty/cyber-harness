package guardrail

import (
	"encoding/json"
	"testing"

	"github.com/chainreactors/cyber/core/resource"
	cfg "github.com/chainreactors/cyber/pkg/config"
)

// The fork keeps JEV screening rules nested under `jev` — the shape Saker writes
// to the operator's cyber.yaml — instead of upstream's flattened Rules embed.
// This pins the location: a nested block decodes, a flat one is rejected as an
// unknown field, and the JSON view keeps the nesting.
func TestNestedJEVRules(t *testing.T) {
	sections := cfg.NewSections()
	resources := resource.New()
	if _, err := resource.Define[cfg.Section](resources, sections); err != nil {
		t.Fatal(err)
	}
	if err := Declare(resources); err != nil {
		t.Fatal(err)
	}
	input := "guardrail:\n  provider: jev\n  mode: safe\n  jev:\n    level: strict\n    on_error: review\n    criteria:\n      review: operator rule\n"
	option := &cfg.Option{Sections: sections}
	if err := cfg.LoadConfigBytes([]byte(input), option); err != nil {
		t.Fatal(err)
	}
	resolved, err := sections.ResolveValues(option.Extensions, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	config, err := cfg.Get[*Config](resolved, ConfigKey)
	if err != nil {
		t.Fatal(err)
	}
	if config.JEV.Level != "strict" || config.JEV.OnError != "review" || config.Mode != ModeSafe || config.JEV.Criteria["review"] != "operator rule" {
		t.Fatalf("policy changed: %+v", config)
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if nested, ok := fields["jev"].(map[string]any); !ok || nested["level"] != "strict" {
		t.Fatalf("display lost the nested JEV rules: %s", data)
	}
	for _, fields := range []map[string]any{
		{"level": "strict"},
		{"on_error": "record"},
		{"jev": map[string]any{"unknown": "strict"}},
		{"jev": map[string]any{"level": "invalid"}},
		{"unknown": "strict"},
	} {
		if _, err := sections.Decode(ConfigKey, fields); err == nil {
			t.Fatalf("invalid rules accepted: %v", fields)
		}
	}
}

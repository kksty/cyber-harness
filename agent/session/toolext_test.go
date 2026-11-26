package session

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/chainreactors/cyber/aop"
)

// The control plane attaches the per-turn tool environment as its own extension;
// the turn decoder must read every value out of it and treat an absent extension
// as "none", so a turn without one inherits nothing.
func TestTurnToolEnvDecodesTheControlPlaneExtension(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"env": map[string]string{
		"SAKER_BATTLE": "http://127.0.0.1:8080/p/p-1/api",
		"SAKER_AUTH":   "Authorization: Bearer tok",
	}})
	if err != nil {
		t.Fatal(err)
	}
	req := &aop.RunTurnRequest{Extensions: []*anypb.Any{{TypeUrl: toolEnvExtensionURL, Value: raw}}}
	env := turnToolEnv(req)
	got := map[string]bool{}
	for _, kv := range env {
		got[kv] = true
	}
	if !got["SAKER_BATTLE=http://127.0.0.1:8080/p/p-1/api"] || !got["SAKER_AUTH=Authorization: Bearer tok"] {
		t.Fatalf("turnToolEnv = %v", env)
	}
	if turnToolEnv(&aop.RunTurnRequest{}) != nil {
		t.Fatal("a request without the extension must yield no environment")
	}
}

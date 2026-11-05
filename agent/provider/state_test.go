package provider

import (
	"context"
	"encoding/json"
	"github.com/chainreactors/cyber/core/telemetry"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The node's first server configuration arrives through Update, and until it
// succeeds the node has no provider at all. A slow first activation must not be
// rejected by the fast replacement window.
func TestUpdateActivatesASlowFirstProvider(t *testing.T) {
	const firstTokenDelay = 6 * time.Second
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(firstTokenDelay)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "slow-first",
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "pong"}, "finish_reason": "stop"},
			},
		})
	}))
	defer srv.Close()
	var state State
	defer state.reset()
	if err := state.Update(context.Background(), ProviderConfig{
		Provider: "openai", BaseURL: srv.URL + "/v1", APIKey: "fixture", Model: "slow-fixture",
	}, telemetry.NopLogger()); err != nil {
		t.Fatalf("Update rejected the first (slow) provider activation: %v", err)
	}
	if p, _ := state.Current(); p == nil {
		t.Fatal("first activation left the node without a provider")
	}
	if state.Health().State != HealthReady {
		t.Fatalf("health = %+v, want ready", state.Health())
	}
}

// A configured endpoint whose first token takes longer than the short liveness
// window is still usable. Reload must keep the provider; otherwise the node
// discards the candidate, stays provider-disabled, and every session it opens
// fails with "agent provider is nil".
func TestReloadKeepsAProviderWhoseEndpointIsSlowToAnswer(t *testing.T) {
	const firstTokenDelay = 6 * time.Second
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(firstTokenDelay)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "slow-probe",
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "pong"}, "finish_reason": "stop"},
			},
		})
	}))
	defer srv.Close()
	var state State
	defer state.reset()
	p, _, err := state.Reload(context.Background(), ProviderConfig{
		Provider: "openai", BaseURL: srv.URL + "/v1", APIKey: "fixture", Model: "slow-fixture",
	}, telemetry.NopLogger())
	if err != nil {
		t.Fatalf("Reload rejected a slow but working endpoint: %v", err)
	}
	if p == nil || state.Health().State != HealthReady {
		t.Fatalf("provider = %v, health = %+v; want the activated provider reported ready", p, state.Health())
	}
}

func TestInitializeRejectsUnknownStartupMode(t *testing.T) {
	var state State
	err := state.initialize(t.Context(), StartupConfig{Mode: StartupMode(99)}, telemetry.NopLogger())
	if err == nil {
		t.Fatal("unknown provider startup mode was accepted")
	}
	if current, config := state.Current(); current != nil || config != (ProviderConfig{}) {
		t.Fatalf("invalid mode changed provider state: %v, %+v", current, config)
	}
}

type borrowedStateProvider struct{ closed bool }

func (*borrowedStateProvider) Name() string { return "borrowed" }
func (*borrowedStateProvider) ChatCompletion(context.Context, *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	return &ChatCompletionResponse{}, nil
}
func (p *borrowedStateProvider) CloseIdleConnections() { p.closed = true }
func TestStateClosesOwnedTransportsAndPreservesBorrowedProvider(t *testing.T) {
	closed := make(chan struct{}, 4)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			closed <- struct{}{}
		}
	}
	server.Start()
	defer server.Close()
	var state State
	defer state.reset()
	p, _, err := state.Reload(t.Context(), ProviderConfig{Provider: "openai", Model: "fixture", APIKey: "fixture", BaseURL: server.URL + "/v1"}, telemetry.NopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.ChatCompletion(t.Context(), &ChatCompletionRequest{}); err != nil {
		t.Fatal(err)
	}
	borrowed := &borrowedStateProvider{}
	state.Set(borrowed, ProviderConfig{Model: "borrowed"})
	state.reset()
	for range 2 {
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
			t.Fatal("probe or retained provider transport did not close")
		}
	}
	if borrowed.closed {
		t.Fatal("closed a borrowed provider")
	}
}

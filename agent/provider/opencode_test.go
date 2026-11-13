package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

type opencodeTransport func(*http.Request) (*http.Response, error)

func (f opencodeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOpenCodeHeadersFollowEachConcurrentConversation(t *testing.T) {
	p, err := NewOpenAIProvider(&ProviderConfig{Provider: "openai", BaseURL: "https://opencode.ai/inference/go/openai/v1/chat/completions/", APIKey: "fixture", Timeout: 5})
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	ids := map[string]int{}
	p.client = &http.Client{Transport: opencodeTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/inference/go/openai/v1/chat/completions" {
			t.Errorf("wrong endpoint: %s", r.URL.Path)
		}
		id := r.Header.Get("x-opencode-session")
		if id == "" || r.UserAgent() != "cyber-harness/0.x" {
			t.Errorf("missing service headers")
		}
		mu.Lock()
		ids[id]++
		mu.Unlock()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)), Header: make(http.Header)}, nil
	})}
	var wg sync.WaitGroup
	for _, id := range []string{"conversation-a", "conversation-b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			for n := 0; n < 2; n++ {
				if _, err := p.ChatCompletion(context.Background(), &ChatCompletionRequest{SessionID: id}); err != nil {
					t.Error(err)
				}
			}
		}(id)
	}
	wg.Wait()
	if ids["conversation-a"] != 2 || ids["conversation-b"] != 2 || len(ids) != 2 {
		t.Fatalf("mixed session headers: %v", ids)
	}
}

func TestOpenCodeStreamingAndAuxiliaryHeaders(t *testing.T) {
	p, err := NewOpenAIProvider(&ProviderConfig{Provider: "openai", BaseURL: "https://opencode.ai/inference/go/openai/v1", Timeout: 5})
	if err != nil {
		t.Fatal(err)
	}
	var auxiliary string
	p.client = &http.Client{Transport: opencodeTransport(func(r *http.Request) (*http.Response, error) {
		id := r.Header.Get("x-opencode-session")
		if id == "" {
			t.Error("empty session header")
		}
		body := `{"data":[{"id":"fixture-model"}]}`
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			if id != "stream-session" {
				t.Errorf("stream id = %q", id)
			}
			body = "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
		} else {
			if r.URL.Path != "/inference/go/openai/v1/models" {
				t.Errorf("catalogue path = %q", r.URL.Path)
			}
			if auxiliary != "" && auxiliary != id {
				t.Error("auxiliary identity changed")
			}
			auxiliary = id
			if id == "stream-session" {
				t.Error("auxiliary request reused conversation")
			}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	for n := 0; n < 2; n++ {
		if _, err := p.ListModels(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	events, err := p.ChatCompletionStream(context.Background(), &ChatCompletionRequest{SessionID: "stream-session"})
	if err != nil {
		t.Fatal(err)
	}
	var done bool
	for ev := range events {
		if ev.Err != nil {
			t.Fatal(ev.Err)
		}
		done = done || ev.Done
	}
	if !done {
		t.Fatal("stream did not complete")
	}
}

func TestOpenCodeSessionHeaderIsNotSentToOtherHosts(t *testing.T) {
	for _, base := range []string{"https://opencode.ai.evil.test/v1", "https://opencode.ai@evil.test/v1", "https://example.test/opencode.ai", "http://opencode.ai/v1"} {
		p, err := NewOpenAIProvider(&ProviderConfig{BaseURL: base})
		if err != nil {
			t.Fatal(err)
		}
		r, _ := http.NewRequest(http.MethodPost, base, nil)
		p.requestHeaders(context.Background(), &ChatCompletionRequest{SessionID: "private-session"})(r)
		if r.Header.Get("x-opencode-session") != "" {
			t.Fatalf("session leaked to %s", base)
		}
	}
}

func TestOpenCodeAuxiliaryRequestUsesOwningConversation(t *testing.T) {
	p, err := NewOpenAIProvider(&ProviderConfig{BaseURL: "https://opencode.ai/inference/go/openai/v1"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithConversationID(context.Background(), "logical-conversation")
	r, _ := http.NewRequest(http.MethodPost, p.completionEndpoint(), nil)
	p.requestHeaders(ctx, nil)(r)
	if r.Header.Get("x-opencode-session") != "logical-conversation" {
		t.Fatal("auxiliary request lost its owning conversation")
	}
	p.requestHeaders(ctx, &ChatCompletionRequest{SessionID: "explicit-child"})(r)
	if r.Header.Get("x-opencode-session") != "explicit-child" {
		t.Fatal("child request inherited parent conversation")
	}
}

func TestOpenCodeLogicalRoutingDoesNotChangePromptCacheIdentity(t *testing.T) {
	p, err := NewOpenAIProvider(&ProviderConfig{BaseURL: "https://opencode.ai/inference/go/openai/v1"})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest(http.MethodPost, p.completionEndpoint(), nil)
	completion := &ChatCompletionRequest{SessionID: "physical", ConversationID: "logical", CacheRetention: CacheShort}
	p.requestHeaders(context.Background(), completion)(r)
	if r.Header.Get("x-opencode-session") != "logical" {
		t.Fatal("routing identity is not logical")
	}
	raw, err := marshalOpenAIRequest(completion)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["prompt_cache_key"] != "physical" {
		t.Fatal("routing changed existing prompt cache key")
	}
}

package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	aop "github.com/chainreactors/cyber/aop"
)

// A 404 on the chat endpoint must surface as an actionable protocol-mismatch
// hint (not a bare "API error (404): " with an empty body, which is what an
// Anthropic-only gateway returns for /chat/completions). The underlying
// *APIError must still unwrap so retry classification is unchanged.
func TestChatCompletion404GivesProtocolHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound) // empty body, like a wrong-protocol gateway
	}))
	defer srv.Close()

	cases := []struct {
		name         string
		provider     string
		wantProvider string // the provider the hint should steer the user toward
	}{
		{"openai endpoint 404 suggests anthropic", "openai", "llm.provider=anthropic"},
		{"anthropic endpoint 404 suggests openai", "anthropic", "llm.provider=openai"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewProvider(&ProviderConfig{Provider: tc.provider, BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"})
			if err != nil {
				t.Fatalf("NewProvider: %v", err)
			}
			_, err = p.ChatCompletion(context.Background(), &ChatCompletionRequest{
				Messages: []*aop.Message{TextMessage("user", "hi")},
			})
			if err == nil {
				t.Fatal("expected a 404 error")
			}
			if !strings.Contains(err.Error(), tc.wantProvider) {
				t.Fatalf("error missing actionable hint %q: %v", tc.wantProvider, err)
			}
			// The wrapped *APIError must remain reachable and non-retryable.
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
				t.Fatalf("underlying *APIError(404) lost after wrapping: %v", err)
			}
			if apiErr.IsRetryable() {
				t.Fatal("a 404 must stay non-retryable")
			}
		})
	}
}

// The official Anthropic endpoint is unambiguous and must infer the anthropic
// protocol even with a blank provider; arbitrary gateways stay openai-default.
// Quota exhaustion can arrive as 429 (some gateways avoid 402 for SDK
// compatibility). It must not be treated as a transient rate limit: waiting
// cannot refill the account, so the agent fails fast and Tianwu's credential
// layer pauses or switches. Plain rate-limit 429 stays retryable.
func TestQuotaExhausted429IsNotRetryable(t *testing.T) {
	quotaBodies := map[string]string{
		"openai type":       `{"error":{"type":"insufficient_quota","message":"You exceeded your current quota"}}`,
		"balance message":   `{"error":{"message":"insufficient balance"}}`,
		"chinese quota":     `{"error":{"message":"额度已用尽，请充值"}}`,
		"chinese low quota": `{"error":{"message":"账户余额不足"}}`,
	}
	for name, body := range quotaBodies {
		t.Run(name, func(t *testing.T) {
			if !QuotaExhaustedMessage(body) {
				t.Fatalf("quota body not recognized: %s", body)
			}
			if (&APIError{StatusCode: 429, Message: body}).IsRetryable() {
				t.Fatalf("quota 429 must not be retryable: %s", body)
			}
		})
	}
	rate := &APIError{StatusCode: 429, Message: `{"error":{"message":"rate limit exceeded"}}`}
	if !rate.IsRetryable() {
		t.Fatal("a plain 429 rate limit must remain retryable")
	}
	if QuotaExhaustedMessage(`rate limit exceeded`) {
		t.Fatal("a plain rate limit must not look like quota exhaustion")
	}
}

func TestInferFromBaseURLAnthropicOfficial(t *testing.T) {
	for _, endpoint := range []string{"https://api.deepseek.com/anthropic", "https://api.deepseek.com/anthropic/v1", "https://api.deepseek.com/anthropic/v1/messages"} {
		if got := InferFromBaseURL(endpoint); got != "anthropic" {
			t.Fatalf("InferFromBaseURL(%q) = %q", endpoint, got)
		}
	}
	if got := InferFromBaseURL("https://api.anthropic.com/v1"); got != "anthropic" {
		t.Fatalf("InferFromBaseURL(api.anthropic.com) = %q, want anthropic", got)
	}
	if got := InferFromBaseURL("https://gateway.example.com/v1"); got != "openai" {
		t.Fatalf("custom gateway must still default to openai, got %q", got)
	}
}

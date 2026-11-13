package provider

import (
	"context"
	"crypto/rand"
	"os"
	"strings"
	"testing"
	"time"

	aop "github.com/chainreactors/cyber/aop"
)

// Explicit opt-in only: credentials stay in the test process environment.
func TestOpenCodeLiveChatAndStream(t *testing.T) {
	key := strings.TrimSpace(os.Getenv("CYBER_TEST_OPENCODE_KEY"))
	if key == "" {
		t.Skip("set CYBER_TEST_OPENCODE_KEY to verify the Go subscription")
	}
	model := os.Getenv("CYBER_TEST_OPENCODE_MODEL")
	if model == "" {
		model = "deepseek-v4.1-flash"
	}
	p, err := NewOpenAIProvider(&ProviderConfig{Provider: "openai", BaseURL: "https://opencode.ai/inference/go/openai/v1", APIKey: key, Model: model, Timeout: 60})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	session := "cyber-test-" + rand.Text()
	req := &ChatCompletionRequest{Model: model, SessionID: session, MaxTokens: 256, Messages: []*aop.Message{TextMessage("user", "软件适配连接验证：计算 2+2，只回复结果。")}}
	res, err := p.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("non-streaming request failed (%T)", err)
	}
	if len(res.Choices) == 0 || strings.TrimSpace(MessageText(res.Choices[0].Message)) == "" {
		t.Fatal("non-streaming response is empty")
	}
	stream, err := p.ChatCompletionStream(ctx, req)
	if err != nil {
		t.Fatalf("streaming request failed (%T)", err)
	}
	text := ""
	done := false
	for ev := range stream {
		if ev.Err != nil {
			t.Fatalf("stream failed (%T)", ev.Err)
		}
		if ev.MessageDelta != nil {
			text += ev.MessageDelta.GetText()
		}
		done = done || ev.Done
	}
	if !done || strings.TrimSpace(text) == "" {
		t.Fatalf("stream incomplete: done=%t text_present=%t", done, text != "")
	}
	t.Logf("OpenCode Go accepted non-streaming and streaming requests for %s with one conversation", model)
}

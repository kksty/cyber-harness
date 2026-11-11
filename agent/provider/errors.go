package provider

import (
	"errors"
	"strings"
)

var (
	ErrCallTimeout      = errors.New("provider call timeout")
	ErrStreamStalled    = errors.New("stream stalled")
	ErrStreamIncomplete = errors.New("stream ended before terminal marker")
)

// QuotaExhaustedMessage reports whether a provider error body says the account
// is out of quota/credit rather than being momentarily throttled. The wire
// contract is text: OpenAI-style {"error":{"type":"insufficient_quota"}},
// gateways that put the same phrase in message, and Chinese "额度已用尽" /
// "余额不足" bodies. Retrying these is pointless — the balance does not refill
// inside a retry budget — so the agent must fail fast and let the control plane
// stop or switch credentials (Tianwu D67/D95).
func QuotaExhaustedMessage(msg string) bool {
	msg = strings.ToLower(msg)
	for _, marker := range []string{
		"insufficient_quota", "insufficient quota",
		"insufficient_balance", "insufficient balance",
		"quota_exceeded", "quota exceeded",
		"credit balance", "billing", "payment required",
		"额度已用尽", "额度用尽", "额度不足",
		"余额不足", "欠费", "配额不足", "配额已用完",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

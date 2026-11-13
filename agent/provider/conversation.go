package provider

import "context"

type conversationKey struct{}

// WithConversationID keeps auxiliary model requests (for example compaction
// and tool parameter generation) on the owning conversation's routing key.
// A child agent must bind its own ID instead of inheriting its parent's value.
func WithConversationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, conversationKey{}, id)
}

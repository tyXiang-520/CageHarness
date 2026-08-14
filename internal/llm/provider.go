package llm

import "context"

// Provider is the interface for LLM providers.
// All providers must implement this interface.
type Provider interface {
	// Generate sends messages to the LLM and returns a response.
	Generate(ctx context.Context, messages []Message) (Response, error)
}

// ProviderFunc is a convenience type for creating a Provider from a function.
type ProviderFunc func(ctx context.Context, messages []Message) (Response, error)

// Generate implements the Provider interface.
func (f ProviderFunc) Generate(ctx context.Context, messages []Message) (Response, error) {
	return f(ctx, messages)
}
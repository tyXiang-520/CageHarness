package llm

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// MockProvider is a test double for LLM Provider.
// It supports two modes:
//   - Sequence mode: returns predefined responses in order
//   - Function mode: uses a Handler function to generate responses dynamically
//
// Function mode takes precedence when a Handler is set.
type MockProvider struct {
	mu        sync.Mutex
	responses []Response
	index     int
	handler   func(messages []Message) (Response, error)
	recorded  [][]Message
}

// NewMockProvider creates a new MockProvider with the given sequence of responses.
// Pass nil or an empty slice to create a provider that only supports function mode.
func NewMockProvider(responses []Response) *MockProvider {
	p := &MockProvider{}
	if responses != nil {
		p.responses = responses
	}
	return p
}

// SetHandler sets the function mode handler.
// When set, the handler is called for each Generate call instead of using the sequence.
func (p *MockProvider) SetHandler(handler func(messages []Message) (Response, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handler = handler
}

// Generate implements the Provider interface.
func (p *MockProvider) Generate(ctx context.Context, messages []Message) (Response, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Record the messages
	p.recorded = append(p.recorded, messages)

	// Function mode takes precedence
	if p.handler != nil {
		return p.handler(messages)
	}

	// Sequence mode
	if p.index >= len(p.responses) {
		return Response{}, fmt.Errorf("mock provider: sequence exhausted (%d/%d)", p.index, len(p.responses))
	}

	resp := p.responses[p.index]
	p.index++
	return resp, nil
}

// Reset resets the sequence index, clearing recorded messages.
// Does not clear the handler.
func (p *MockProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.index = 0
	p.recorded = nil
}

// Messages returns all recorded message batches.
func (p *MockProvider) Messages() [][]Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([][]Message, len(p.recorded))
	copy(result, p.recorded)
	return result
}

// Ensure MockProvider implements Provider at compile time.
var _ Provider = (*MockProvider)(nil)

// Common errors
var (
	ErrProviderExhausted = errors.New("mock provider: sequence exhausted")
	ErrProviderHandler   = errors.New("mock provider: handler error")
)
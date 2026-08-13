package credential

import (
	"fmt"
	"sync"
)

// MockProvider is an in-memory credential provider for testing.
type MockProvider struct {
	mu    sync.RWMutex
	store map[string]string
}

// NewMockProvider creates a new MockProvider.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		store: make(map[string]string),
	}
}

// Get retrieves a credential value by key.
func (p *MockProvider) Get(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("credential: empty key")
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	val, ok := p.store[key]
	if !ok {
		return "", fmt.Errorf("credential: key %q not found: %w", key, ErrNotFound)
	}
	return val, nil
}

// Set stores a credential value.
func (p *MockProvider) Set(key, value string) error {
	if key == "" {
		return fmt.Errorf("credential: empty key")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.store[key] = value
	return nil
}

// Delete removes a credential by key.
func (p *MockProvider) Delete(key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.store, key)
	return nil
}

// Reset clears all stored credentials.
func (p *MockProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.store = make(map[string]string)
}

// Ensure MockProvider implements Provider at compile time.
var _ Provider = (*MockProvider)(nil)
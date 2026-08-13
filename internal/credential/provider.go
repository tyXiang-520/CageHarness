package credential

// Provider is the interface for credential storage.
// Supports Keychain (Phase 2), EnvProvider, and MockProvider.
type Provider interface {
	// Get retrieves a credential value by key.
	// Returns ErrNotFound if the key doesn't exist.
	Get(key string) (string, error)

	// Set stores a credential value.
	Set(key, value string) error

	// Delete removes a credential by key.
	// Returns nil if the key doesn't exist (idempotent).
	Delete(key string) error
}
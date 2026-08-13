package credential

import (
	"fmt"
	"os"
)

// EnvProvider reads credentials from environment variables.
// This is a simple, non-secure provider suitable for development.
// For production, use OS Keychain (Phase 2).
type EnvProvider struct{}

// NewEnvProvider creates a new EnvProvider.
func NewEnvProvider() *EnvProvider {
	return &EnvProvider{}
}

// Get retrieves a credential value from the environment variable with the given key.
func (p *EnvProvider) Get(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("credential: empty key")
	}

	val := os.Getenv(key)
	if val == "" {
		// Check if the variable exists but is empty
		_, exists := os.LookupEnv(key)
		if !exists {
			return "", fmt.Errorf("credential: key %q not found in environment: %w", key, ErrNotFound)
		}
	}
	return val, nil
}

// Set stores a credential value as an environment variable.
func (p *EnvProvider) Set(key, value string) error {
	if key == "" {
		return fmt.Errorf("credential: empty key")
	}

	if err := os.Setenv(key, value); err != nil {
		return fmt.Errorf("credential: setenv %q: %w", key, err)
	}
	return nil
}

// Delete removes a credential from the environment.
func (p *EnvProvider) Delete(key string) error {
	if err := os.Unsetenv(key); err != nil {
		return fmt.Errorf("credential: unsetenv %q: %w", key, err)
	}
	return nil
}

// Ensure EnvProvider implements Provider at compile time.
var _ Provider = (*EnvProvider)(nil)
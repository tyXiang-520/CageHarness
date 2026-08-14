package credential

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// ErrNotFound is returned when a credential is not found.
var ErrNotFound = errors.New("credential not found")

// RedactedMarker is the placeholder text used when redacting sensitive values.
const RedactedMarker = "[REDACTED]"

// CredentialStore is the interface for secure credential management.
//
// Implementations:
//   - MockStore: in-memory store for testing
//   - EnvStore: environment variable store with .env file support
//   - KeychainStore: OS keychain integration (deferred to Phase 2)
type CredentialStore interface {
	// Set stores a credential value for the given name.
	Set(name, secret string) error

	// Get retrieves a credential value by name.
	// Returns ErrNotFound if the credential does not exist.
	Get(name string) (string, error)

	// Delete removes a credential.
	// Returns ErrNotFound if the credential does not exist.
	Delete(name string) error

	// Exists returns true if a credential exists for the given name.
	Exists(name string) bool
}

// =============================================================================
// MockStore — in-memory credential store for testing
// =============================================================================

// MockStore is an in-memory implementation of CredentialStore for testing.
// It never persists credentials to disk.
type MockStore struct {
	mu    sync.RWMutex
	store map[string]string
}

// NewMockStore creates a new empty MockStore.
func NewMockStore() *MockStore {
	return &MockStore{
		store: make(map[string]string),
	}
}

// Set stores a credential value in memory.
func (m *MockStore) Set(name, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[name] = secret
	return nil
}

// Get retrieves a credential value from memory.
func (m *MockStore) Get(name string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.store[name]
	if !ok {
		return "", ErrNotFound
	}
	return val, nil
}

// Delete removes a credential from memory.
func (m *MockStore) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store[name]; !ok {
		return ErrNotFound
	}
	delete(m.store, name)
	return nil
}

// Exists checks if a credential exists in memory.
func (m *MockStore) Exists(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.store[name]
	return ok
}

// compile-time interface check
var _ CredentialStore = (*MockStore)(nil)

// =============================================================================
// EnvStore — environment variable credential store
// =============================================================================

// EnvStore reads credentials from environment variables.
// It supports loading from .env files as a compatibility input source.
// Set operations write to process environment variables via os.Setenv.
//
// Security note: .env files are plaintext. This is a known risk documented
// in the project README. For production use, prefer OS keychain integration.
type EnvStore struct {
	mu   sync.RWMutex
	vars map[string]string // tracks variables set via Set() or loaded from .env
}

// NewEnvStore creates a new EnvStore that reads from the current environment.
func NewEnvStore() *EnvStore {
	return &EnvStore{
		vars: make(map[string]string),
	}
}

// Set stores a credential as an environment variable.
func (e *EnvStore) Set(name, secret string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := os.Setenv(name, secret); err != nil {
		return fmt.Errorf("setenv %s: %w", name, err)
	}
	e.vars[name] = secret
	return nil
}

// Get retrieves a credential from the environment.
// Checks environment variables first, then .env-loaded values.
func (e *EnvStore) Get(name string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Check OS environment first
	if val, ok := os.LookupEnv(name); ok {
		return val, nil
	}

	// Check .env-loaded values
	if val, ok := e.vars[name]; ok {
		return val, nil
	}

	return "", ErrNotFound
}

// Delete removes a credential from the environment.
func (e *EnvStore) Delete(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := os.LookupEnv(name); !ok {
		if _, ok := e.vars[name]; !ok {
			return ErrNotFound
		}
	}

	if err := os.Unsetenv(name); err != nil {
		return fmt.Errorf("unsetenv %s: %w", name, err)
	}
	delete(e.vars, name)
	return nil
}

// Exists checks if a credential exists in the environment.
func (e *EnvStore) Exists(name string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, ok := os.LookupEnv(name); ok {
		return true
	}
	_, ok := e.vars[name]
	return ok
}

// LoadDotEnv loads credentials from a .env file.
// Environment variables take priority over .env values — if a key
// already exists in the environment, the .env value is not loaded.
// Lines starting with '#' are treated as comments and ignored.
// Empty lines are skipped.
func (e *EnvStore) LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open .env file: %w", err)
	}
	defer file.Close()

	e.mu.Lock()
	defer e.mu.Unlock()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if present
		value = strings.Trim(value, `"'`)

		// Only set if not already in the OS environment
		if _, ok := os.LookupEnv(key); !ok {
			e.vars[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan .env file: %w", err)
	}

	return nil
}

// compile-time interface check
var _ CredentialStore = (*EnvStore)(nil)

// =============================================================================
// RedactSensitiveFields — audit log redaction
// =============================================================================

// sensitivePrefixes are field name patterns that should be redacted.
// Matching is case-insensitive and based on substring containment.
var sensitivePrefixes = []string{
	"api_key", "apikey", "token", "secret", "password", "credential",
	"authorization", "auth",
}

// RedactSensitiveFields replaces sensitive field values with RedactedMarker.
// It recursively processes nested maps. The original map is not mutated.
//
// Sensitive fields are identified by case-insensitive substring matching
// against sensitivePrefixes (e.g., "my_api_key_v2" matches "api_key").
func RedactSensitiveFields(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}

	result := make(map[string]any, len(params))
	for key, value := range params {
		if isSensitiveField(key) {
			result[key] = RedactedMarker
		} else if nested, ok := value.(map[string]any); ok {
			result[key] = RedactSensitiveFields(nested)
		} else {
			result[key] = value
		}
	}
	return result
}

// isSensitiveField checks if a field name matches any sensitive prefix.
func isSensitiveField(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range sensitivePrefixes {
		if strings.Contains(lower, prefix) {
			return true
		}
	}
	return false
}
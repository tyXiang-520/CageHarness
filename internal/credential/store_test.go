package credential

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// MockStore Tests
// =============================================================================

func TestMockStore_SetAndGet(t *testing.T) {
	store := NewMockStore()

	err := store.Set("OPENAI_API_KEY", "sk-test-123")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := store.Get("OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "sk-test-123" {
		t.Errorf("expected 'sk-test-123', got '%s'", val)
	}
}

func TestMockStore_GetNotFound(t *testing.T) {
	store := NewMockStore()

	_, err := store.Get("NONEXISTENT")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockStore_Exists(t *testing.T) {
	store := NewMockStore()

	if store.Exists("OPENAI_API_KEY") {
		t.Error("key should not exist before Set")
	}

	store.Set("OPENAI_API_KEY", "sk-test-123")

	if !store.Exists("OPENAI_API_KEY") {
		t.Error("key should exist after Set")
	}
}

func TestMockStore_Delete(t *testing.T) {
	store := NewMockStore()

	store.Set("OPENAI_API_KEY", "sk-test-123")
	err := store.Delete("OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if store.Exists("OPENAI_API_KEY") {
		t.Error("key should not exist after Delete")
	}
}

func TestMockStore_DeleteNotFound(t *testing.T) {
	store := NewMockStore()

	err := store.Delete("NONEXISTENT")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockStore_Overwrite(t *testing.T) {
	store := NewMockStore()

	store.Set("KEY", "old-value")
	store.Set("KEY", "new-value")

	val, _ := store.Get("KEY")
	if val != "new-value" {
		t.Errorf("expected 'new-value', got '%s'", val)
	}
}

// =============================================================================
// EnvStore Tests
// =============================================================================

func TestEnvStore_GetFromEnv(t *testing.T) {
	os.Setenv("TEST_ENV_KEY", "env-value")
	defer os.Unsetenv("TEST_ENV_KEY")

	store := NewEnvStore()
	val, err := store.Get("TEST_ENV_KEY")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "env-value" {
		t.Errorf("expected 'env-value', got '%s'", val)
	}
}

func TestEnvStore_GetNotFound(t *testing.T) {
	store := NewEnvStore()

	_, err := store.Get("NONEXISTENT_ENV_KEY_12345")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEnvStore_Exists(t *testing.T) {
	os.Setenv("TEST_EXISTS_KEY", "value")
	defer os.Unsetenv("TEST_EXISTS_KEY")

	store := NewEnvStore()
	if !store.Exists("TEST_EXISTS_KEY") {
		t.Error("key should exist in environment")
	}
	if store.Exists("NONEXISTENT_KEY_12345") {
		t.Error("key should not exist")
	}
}

func TestEnvStore_SetAndGet(t *testing.T) {
	store := NewEnvStore()

	err := store.Set("TEST_SET_KEY", "set-value")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := store.Get("TEST_SET_KEY")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "set-value" {
		t.Errorf("expected 'set-value', got '%s'", val)
	}
}

func TestEnvStore_Delete(t *testing.T) {
	store := NewEnvStore()
	store.Set("TEST_DEL_KEY", "value")

	err := store.Delete("TEST_DEL_KEY")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if store.Exists("TEST_DEL_KEY") {
		t.Error("key should not exist after Delete")
	}

	// Clean up env var if the store didn't
	os.Unsetenv("TEST_DEL_KEY")
}

func TestEnvStore_LoadDotEnv(t *testing.T) {
	// Create a temporary .env file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := "DOTENV_KEY=dotenv-value\n# comment line\nANOTHER_KEY=another-value\n"
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	store := NewEnvStore()
	if err := store.LoadDotEnv(envFile); err != nil {
		t.Fatalf("LoadDotEnv failed: %v", err)
	}

	val, err := store.Get("DOTENV_KEY")
	if err != nil {
		t.Fatalf("Get DOTENV_KEY failed: %v", err)
	}
	if val != "dotenv-value" {
		t.Errorf("expected 'dotenv-value', got '%s'", val)
	}

	val2, _ := store.Get("ANOTHER_KEY")
	if val2 != "another-value" {
		t.Errorf("expected 'another-value', got '%s'", val2)
	}
}

func TestEnvStore_LoadDotEnv_FileNotFound(t *testing.T) {
	store := NewEnvStore()
	err := store.LoadDotEnv("/nonexistent/path/.env")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestEnvStore_DotEnvDoesNotOverrideEnv(t *testing.T) {
	os.Setenv("PRIORITY_KEY", "env-value")
	defer os.Unsetenv("PRIORITY_KEY")

	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	os.WriteFile(envFile, []byte("PRIORITY_KEY=dotenv-value\n"), 0644)

	store := NewEnvStore()
	store.LoadDotEnv(envFile)

	val, _ := store.Get("PRIORITY_KEY")
	// EnvStore should NOT override existing env vars with .env values
	if val != "env-value" {
		t.Errorf("expected 'env-value' (env var takes priority), got '%s'", val)
	}
}

// =============================================================================
// RedactSensitiveFields Tests
// =============================================================================

func TestRedactSensitiveFields_BasicRedaction(t *testing.T) {
	params := map[string]any{
		"command": "echo hello",
		"token":   "sk-secret-token-123",
	}

	result := RedactSensitiveFields(params)

	if result["command"] != "echo hello" {
		t.Errorf("expected 'echo hello', got '%v'", result["command"])
	}
	if result["token"] != RedactedMarker {
		t.Errorf("expected '%s', got '%v'", RedactedMarker, result["token"])
	}
}

func TestRedactSensitiveFields_MultipleFields(t *testing.T) {
	params := map[string]any{
		"api_key":      "sk-abc123",
		"password":     "super-secret",
		"authorization": "Bearer token-xyz",
		"secret":       "my-secret",
		"credential":   "cred-data",
		"normal_field": "normal-value",
	}

	result := RedactSensitiveFields(params)

	sensitiveKeys := []string{"api_key", "password", "authorization", "secret", "credential"}
	for _, key := range sensitiveKeys {
		if result[key] != RedactedMarker {
			t.Errorf("key '%s' should be redacted, got '%v'", key, result[key])
		}
	}
	if result["normal_field"] != "normal-value" {
		t.Errorf("normal_field should not be redacted, got '%v'", result["normal_field"])
	}
}

func TestRedactSensitiveFields_CaseInsensitive(t *testing.T) {
	params := map[string]any{
		"API_KEY":       "sk-upper",
		"Authorization": "Bearer upper-auth",
		"Secret":        "upper-secret",
	}

	result := RedactSensitiveFields(params)

	for key := range params {
		if result[key] != RedactedMarker {
			t.Errorf("key '%s' should be redacted (case insensitive), got '%v'", key, result[key])
		}
	}
}

func TestRedactSensitiveFields_NestedMap(t *testing.T) {
	params := map[string]any{
		"headers": map[string]any{
			"Authorization": "Bearer nested-token",
			"Content-Type":  "application/json",
		},
	}

	result := RedactSensitiveFields(params)

	headers, ok := result["headers"].(map[string]any)
	if !ok {
		t.Fatal("headers should still be a map")
	}
	if headers["Authorization"] != RedactedMarker {
		t.Errorf("nested Authorization should be redacted, got '%v'", headers["Authorization"])
	}
	if headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type should not be redacted, got '%v'", headers["Content-Type"])
	}
}

func TestRedactSensitiveFields_NoSensitiveData(t *testing.T) {
	params := map[string]any{
		"command": "go test ./...",
		"path":    "/tmp/workspace",
		"timeout": 30,
	}

	result := RedactSensitiveFields(params)

	for key, expected := range params {
		if result[key] != expected {
			t.Errorf("key '%s': expected '%v', got '%v'", key, expected, result[key])
		}
	}
}

func TestRedactSensitiveFields_NilInput(t *testing.T) {
	result := RedactSensitiveFields(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestRedactSensitiveFields_EmptyInput(t *testing.T) {
	result := RedactSensitiveFields(map[string]any{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestRedactSensitiveFields_ContainsSensitiveSubstring(t *testing.T) {
	params := map[string]any{
		"my_api_key_v2": "sk-substring-match",
		"user_token":    "tok-substring",
		"secret_key":    "sec-substring",
	}

	result := RedactSensitiveFields(params)

	for key := range params {
		if result[key] != RedactedMarker {
			t.Errorf("key '%s' should be redacted (substring match), got '%v'", key, result[key])
		}
	}
}
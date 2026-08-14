package credential

import (
	"runtime"
	"testing"
)

func TestKeychainStore_New(t *testing.T) {
	ks := NewKeychainStore()
	if ks == nil {
		t.Fatal("NewKeychainStore returned nil")
	}
	if ks.service != "CageHarness" {
		t.Errorf("service = %q, want %q", ks.service, "CageHarness")
	}
	// Whether it's available depends on the OS, but it should not panic
	_ = ks.IsAvailable()
}

func TestKeychainStore_Interface(t *testing.T) {
	var ks CredentialStore = NewKeychainStore()
	_ = ks
}

func TestKeychainStore_UnavailableOperations(t *testing.T) {
	// Skip if keychain is available — this test is for the unavailable path
	ks := NewKeychainStore()
	if ks.IsAvailable() {
		t.Skip("keychain is available on this system; skipping unavailable-path test")
	}

	if err := ks.Set("test", "secret"); err != ErrKeychainUnavailable {
		t.Errorf("Set on unavailable keychain: got %v, want ErrKeychainUnavailable", err)
	}
	if _, err := ks.Get("test"); err != ErrKeychainUnavailable {
		t.Errorf("Get on unavailable keychain: got %v, want ErrKeychainUnavailable", err)
	}
	if err := ks.Delete("test"); err != ErrKeychainUnavailable {
		t.Errorf("Delete on unavailable keychain: got %v, want ErrKeychainUnavailable", err)
	}
	if ks.Exists("test") {
		t.Error("Exists on unavailable keychain should return false")
	}
}

func TestKeychainStore_AvailableOperations(t *testing.T) {
	ks := NewKeychainStore()
	if !ks.IsAvailable() {
		t.Skip("keychain not available on this system")
	}

	const testName = "CageHarness_Test_Credential"
	const testSecret = "test-secret-value-12345"

	// Clean up any leftover from previous test runs
	_ = ks.Delete(testName)

	// Should not exist yet
	if ks.Exists(testName) {
		ks.Delete(testName)
	}

	// Set
	if err := ks.Set(testName, testSecret); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	t.Cleanup(func() { ks.Delete(testName) })

	// Exists
	if !ks.Exists(testName) {
		t.Error("Exists should return true after Set")
	}

	// Get — on Windows, returns a marker (keychain is write-only)
	got, err := ks.Get(testName)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if runtime.GOOS == "windows" {
		// Windows cmdkey cannot retrieve plaintext passwords
		if got != keychainGetMarker {
			t.Errorf("Get on Windows should return marker, got %q", got)
		}
	} else {
		if got != testSecret {
			t.Errorf("Get returned %q, want %q", got, testSecret)
		}
	}

	// Delete
	if err := ks.Delete(testName); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should not exist after delete
	if ks.Exists(testName) {
		t.Error("Exists should return false after Delete")
	}

	// Get after delete should fail
	if _, err := ks.Get(testName); err == nil {
		t.Error("Get after Delete should return error")
	}
}

func TestKeychainStore_Overwrite(t *testing.T) {
	ks := NewKeychainStore()
	if !ks.IsAvailable() {
		t.Skip("keychain not available on this system")
	}

	const testName = "CageHarness_Test_Overwrite"

	// Clean up
	_ = ks.Delete(testName)
	t.Cleanup(func() { ks.Delete(testName) })

	// Set first value
	if err := ks.Set(testName, "first-secret"); err != nil {
		t.Fatalf("Set (first) failed: %v", err)
	}

	// Overwrite with second value
	if err := ks.Set(testName, "second-secret-updated"); err != nil {
		t.Fatalf("Set (second) failed: %v", err)
	}

	// Should still exist
	if !ks.Exists(testName) {
		t.Error("Exists should return true after overwrite")
	}
}

func TestKeychainStore_DeleteNonExistent(t *testing.T) {
	ks := NewKeychainStore()
	if !ks.IsAvailable() {
		t.Skip("keychain not available on this system")
	}

	err := ks.Delete("CageHarness_NonExistent_Key_xyz")
	if err == nil {
		t.Error("Delete of non-existent key should return an error")
	}
}

func TestReadPassword(t *testing.T) {
	// We can't actually test ReadPassword in an automated test
	// because it requires interactive input. Just verify the function exists.
	t.Skip("ReadPassword requires interactive terminal — skipping automated test")
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"", "(not set)"},
		{"abc", "***"},
		{"shortkey123", "***********"},
		{"sk-1234567890abcdefghij", "sk-1***************ghij"},
		{"sk-1234567890ab", "sk-1*******90ab"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := MaskKey(tt.key)
			if got != tt.want {
				t.Errorf("MaskKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestRedactSensitiveFields(t *testing.T) {
	// Verify RedactSensitiveFields works correctly with key-like values
	params := map[string]any{
		"api_key":         "sk-secret-123",
		"safe_param":      "hello",
		"token":           "ghp_token",
		"nested": map[string]any{
			"password": "secret123",
			"public":   "visible",
		},
	}

	result := RedactSensitiveFields(params)

	if result["api_key"] != RedactedMarker {
		t.Errorf("api_key should be redacted, got %v", result["api_key"])
	}
	if result["token"] != RedactedMarker {
		t.Errorf("token should be redacted, got %v", result["token"])
	}
	if result["safe_param"] != "hello" {
		t.Errorf("safe_param should not be redacted, got %v", result["safe_param"])
	}

	nested, ok := result["nested"].(map[string]any)
	if !ok {
		t.Fatal("nested value should be map[string]any")
	}
	if nested["password"] != RedactedMarker {
		t.Errorf("nested password should be redacted, got %v", nested["password"])
	}
	if nested["public"] != "visible" {
		t.Errorf("nested public should not be redacted, got %v", nested["public"])
	}
}

func TestCredentialStore_AllImplementations(t *testing.T) {
	// Verify all three implementations satisfy the interface
	implementations := []struct {
		name  string
		store CredentialStore
	}{
		{"MockStore", NewMockStore()},
		{"EnvStore", NewEnvStore()},
		{"KeychainStore", NewKeychainStore()},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			if impl.store == nil {
				t.Errorf("%s returned nil", impl.name)
			}
		})
	}
}
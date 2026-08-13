package credential

import (
	"testing"
)

func TestProvider_InterfaceContract(t *testing.T) {
	// Verify MockProvider implements Provider
	var p Provider = NewMockProvider()
	if p == nil {
		t.Fatal("MockProvider should not be nil")
	}

	// Verify EnvProvider implements Provider
	var p2 Provider = NewEnvProvider()
	if p2 == nil {
		t.Fatal("EnvProvider should not be nil")
	}
}

func TestMockProvider_GetSet(t *testing.T) {
	p := NewMockProvider()

	// Set a value
	if err := p.Set("API_KEY", "sk-test123"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the value
	val, err := p.Get("API_KEY")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "sk-test123" {
		t.Errorf("Get = %q, want %q", val, "sk-test123")
	}
}

func TestMockProvider_GetNotFound(t *testing.T) {
	p := NewMockProvider()
	_, err := p.Get("NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for non-existent key")
	}
}

func TestMockProvider_Delete(t *testing.T) {
	p := NewMockProvider()
	p.Set("KEY", "value")
	if err := p.Delete("KEY"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, err := p.Get("KEY")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestMockProvider_DeleteNotFound(t *testing.T) {
	p := NewMockProvider()
	// Deleting a non-existent key should not error
	if err := p.Delete("NONEXISTENT"); err != nil {
		t.Errorf("unexpected error for deleting non-existent key: %v", err)
	}
}

func TestMockProvider_Overwrite(t *testing.T) {
	p := NewMockProvider()
	p.Set("KEY", "value1")
	p.Set("KEY", "value2")

	val, err := p.Get("KEY")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value2" {
		t.Errorf("Get = %q, want %q", val, "value2")
	}
}

func TestMockProvider_MultipleKeys(t *testing.T) {
	p := NewMockProvider()
	p.Set("KEY1", "val1")
	p.Set("KEY2", "val2")
	p.Set("KEY3", "val3")

	v1, _ := p.Get("KEY1")
	v2, _ := p.Get("KEY2")
	v3, _ := p.Get("KEY3")

	if v1 != "val1" || v2 != "val2" || v3 != "val3" {
		t.Errorf("got %q, %q, %q, want %q, %q, %q", v1, v2, v3, "val1", "val2", "val3")
	}
}

func TestEnvProvider_Get(t *testing.T) {
	p := NewEnvProvider()

	// PATH should always exist on any system
	val, err := p.Get("PATH")
	if err != nil {
		t.Fatalf("Get(PATH) failed: %v", err)
	}
	if val == "" {
		t.Error("PATH should not be empty")
	}
}

func TestEnvProvider_GetNotFound(t *testing.T) {
	p := NewEnvProvider()
	_, err := p.Get("HARNESS_NONEXISTENT_ENV_VAR_12345")
	if err == nil {
		t.Fatal("expected error for non-existent env var")
	}
}

func TestEnvProvider_Set(t *testing.T) {
	p := NewEnvProvider()
	if err := p.Set("HARNESS_TEST_VAR", "test_value"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := p.Get("HARNESS_TEST_VAR")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "test_value" {
		t.Errorf("Get = %q, want %q", val, "test_value")
	}

	// Clean up
	p.Delete("HARNESS_TEST_VAR")
}

func TestEnvProvider_Delete(t *testing.T) {
	p := NewEnvProvider()
	p.Set("HARNESS_TEST_DELETE", "to_delete")
	p.Delete("HARNESS_TEST_DELETE")

	_, err := p.Get("HARNESS_TEST_DELETE")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestMockProvider_Reset(t *testing.T) {
	p := NewMockProvider()
	p.Set("KEY", "value")
	p.Reset()

	_, err := p.Get("KEY")
	if err == nil {
		t.Fatal("expected error after reset")
	}
}

func TestEnvProvider_EmptyKey(t *testing.T) {
	p := NewEnvProvider()
	_, err := p.Get("")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestMockProvider_EmptyKey(t *testing.T) {
	p := NewMockProvider()
	_, err := p.Get("")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfig_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LLM.Endpoint != "https://api.openai.com/v1" {
		t.Errorf("LLM.Endpoint = %q, want %q", cfg.LLM.Endpoint, "https://api.openai.com/v1")
	}
	if cfg.LLM.Model != "gpt-4o" {
		t.Errorf("LLM.Model = %q, want %q", cfg.LLM.Model, "gpt-4o")
	}
	if cfg.LLM.MaxTokens != 4096 {
		t.Errorf("LLM.MaxTokens = %d, want %d", cfg.LLM.MaxTokens, 4096)
	}
	if cfg.LLM.Timeout != 60*time.Second {
		t.Errorf("LLM.Timeout = %v, want %v", cfg.LLM.Timeout, 60*time.Second)
	}
	if cfg.Agent.MaxIterations != 10 {
		t.Errorf("Agent.MaxIterations = %d, want %d", cfg.Agent.MaxIterations, 10)
	}
	if cfg.Agent.RunTimeout != 300*time.Second {
		t.Errorf("Agent.RunTimeout = %v, want %v", cfg.Agent.RunTimeout, 300*time.Second)
	}
	if !cfg.Governance.Enabled {
		t.Error("Governance.Enabled should be true by default")
	}
	if cfg.Governance.HITLTimeout != 300*time.Second {
		t.Errorf("Governance.HITLTimeout = %v, want %v", cfg.Governance.HITLTimeout, 300*time.Second)
	}
	if cfg.Governance.ToolTimeout != 30*time.Second {
		t.Errorf("Governance.ToolTimeout = %v, want %v", cfg.Governance.ToolTimeout, 30*time.Second)
	}
	if cfg.Web.Port != 8080 {
		t.Errorf("Web.Port = %d, want %d", cfg.Web.Port, 8080)
	}
}

func TestConfig_LoadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
llm:
  endpoint: "https://custom.endpoint.com/v1"
  model: "gpt-4o-mini"
  max_tokens: 2048
  timeout: 30s

agent:
  max_iterations: 5
  run_timeout: 120s

governance:
  enabled: false
  hitl_timeout: 60s
  tool_timeout: 15s
  workspace_root: "/tmp/work"
  rules:
    - GIT-001
    - SHELL-001

web:
  port: 9090
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if cfg.LLM.Endpoint != "https://custom.endpoint.com/v1" {
		t.Errorf("LLM.Endpoint = %q, want %q", cfg.LLM.Endpoint, "https://custom.endpoint.com/v1")
	}
	if cfg.LLM.Model != "gpt-4o-mini" {
		t.Errorf("LLM.Model = %q, want %q", cfg.LLM.Model, "gpt-4o-mini")
	}
	if cfg.LLM.MaxTokens != 2048 {
		t.Errorf("LLM.MaxTokens = %d, want %d", cfg.LLM.MaxTokens, 2048)
	}
	if cfg.LLM.Timeout != 30*time.Second {
		t.Errorf("LLM.Timeout = %v, want %v", cfg.LLM.Timeout, 30*time.Second)
	}
	if cfg.Agent.MaxIterations != 5 {
		t.Errorf("Agent.MaxIterations = %d, want %d", cfg.Agent.MaxIterations, 5)
	}
	if cfg.Governance.Enabled {
		t.Error("Governance.Enabled should be false")
	}
	if len(cfg.Governance.Rules) != 2 {
		t.Errorf("Governance.Rules = %v, want 2 rules", cfg.Governance.Rules)
	}
	if cfg.Web.Port != 9090 {
		t.Errorf("Web.Port = %d, want %d", cfg.Web.Port, 9090)
	}
}

func TestConfig_LoadFromFileNotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestConfig_LoadFromEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	// Empty file with no content should be parseable but return defaults
	content := ""
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("unexpected error for empty file: %v", err)
	}
	// Empty file should produce defaults
	if cfg.LLM.Endpoint != "https://api.openai.com/v1" {
		t.Errorf("expected default endpoint, got %q", cfg.LLM.Endpoint)
	}
}

func TestConfig_EnvOverrides(t *testing.T) {
	cfg := DefaultConfig()

	// Apply env overrides (simulated)
	os.Setenv("HARNESS_LLM_ENDPOINT", "https://env.endpoint.com/v1")
	os.Setenv("HARNESS_LLM_MODEL", "gpt-4-turbo")
	os.Setenv("HARNESS_AGENT_MAX_ITERATIONS", "20")
	os.Setenv("HARNESS_GOVERNANCE_ENABLED", "false")
	os.Setenv("HARNESS_WEB_PORT", "3000")
	defer func() {
		os.Unsetenv("HARNESS_LLM_ENDPOINT")
		os.Unsetenv("HARNESS_LLM_MODEL")
		os.Unsetenv("HARNESS_AGENT_MAX_ITERATIONS")
		os.Unsetenv("HARNESS_GOVERNANCE_ENABLED")
		os.Unsetenv("HARNESS_WEB_PORT")
	}()

	cfg.ApplyEnvOverrides()

	if cfg.LLM.Endpoint != "https://env.endpoint.com/v1" {
		t.Errorf("LLM.Endpoint = %q, want %q", cfg.LLM.Endpoint, "https://env.endpoint.com/v1")
	}
	if cfg.LLM.Model != "gpt-4-turbo" {
		t.Errorf("LLM.Model = %q, want %q", cfg.LLM.Model, "gpt-4-turbo")
	}
	if cfg.Agent.MaxIterations != 20 {
		t.Errorf("Agent.MaxIterations = %d, want %d", cfg.Agent.MaxIterations, 20)
	}
	if cfg.Governance.Enabled {
		t.Error("Governance.Enabled should be false after env override")
	}
	if cfg.Web.Port != 3000 {
		t.Errorf("Web.Port = %d, want %d", cfg.Web.Port, 3000)
	}
}

func TestConfig_LoadAndApply(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
llm:
  model: "gpt-4o-mini"
web:
  port: 9090
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	// File values should override defaults
	if cfg.LLM.Model != "gpt-4o-mini" {
		t.Errorf("LLM.Model = %q, want %q", cfg.LLM.Model, "gpt-4o-mini")
	}

	// Defaults should be preserved for unset fields
	if cfg.LLM.Endpoint != "https://api.openai.com/v1" {
		t.Errorf("LLM.Endpoint = %q, want %q", cfg.LLM.Endpoint, "https://api.openai.com/v1")
	}
	if cfg.Agent.MaxIterations != 10 {
		t.Errorf("Agent.MaxIterations = %d, want %d", cfg.Agent.MaxIterations, 10)
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := DefaultConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected validation error: %v", err)
		}
	})

	t.Run("empty endpoint", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.LLM.Endpoint = ""
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for empty endpoint")
		}
	})

	t.Run("empty model", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.LLM.Model = ""
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for empty model")
		}
	})

	t.Run("negative max tokens", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.LLM.MaxTokens = -1
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for negative max tokens")
		}
	})

	t.Run("zero max iterations", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Agent.MaxIterations = 0
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for zero max iterations")
		}
	})

	t.Run("negative port", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Web.Port = -1
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for negative port")
		}
	})

	t.Run("port out of range", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Web.Port = 65536
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for port > 65535")
		}
	})
}

func TestConfig_LoadDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() should not return nil")
	}
}

func TestConfig_YAMLSerialization(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LLM.Model = "gpt-4-custom"
	cfg.Web.Port = 3000

	yamlBytes, err := cfg.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML failed: %v", err)
	}

	// Verify the YAML contains our values
	yamlStr := string(yamlBytes)
	if !contains(yamlStr, "gpt-4-custom") {
		t.Error("YAML should contain custom model name")
	}
	if !contains(yamlStr, "3000") {
		t.Error("YAML should contain custom port")
	}

	// Verify it can be parsed back
	cfg2, err := LoadYAML(yamlBytes)
	if err != nil {
		t.Fatalf("LoadYAML failed: %v", err)
	}
	if cfg2.LLM.Model != "gpt-4-custom" {
		t.Errorf("LLM.Model = %q, want %q", cfg2.LLM.Model, "gpt-4-custom")
	}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && containsInner(s, substr))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
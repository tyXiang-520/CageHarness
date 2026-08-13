package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// LLMConfig holds the LLM provider configuration.
type LLMConfig struct {
	Endpoint  string        `yaml:"endpoint"`
	Model     string        `yaml:"model"`
	MaxTokens int           `yaml:"max_tokens"`
	Timeout   time.Duration `yaml:"timeout"`
}

// AgentConfig holds the Agent loop configuration.
type AgentConfig struct {
	MaxIterations int           `yaml:"max_iterations"`
	RunTimeout    time.Duration `yaml:"run_timeout"`
}

// RuleConfig holds a single governance rule identifier.
type RuleConfig struct {
	ID string `yaml:"id"`
}

// GovernanceConfig holds the governance pipeline configuration.
type GovernanceConfig struct {
	Enabled       bool          `yaml:"enabled"`
	HITLTimeout   time.Duration `yaml:"hitl_timeout"`
	ToolTimeout   time.Duration `yaml:"tool_timeout"`
	WorkspaceRoot string        `yaml:"workspace_root"`
	Rules         []string      `yaml:"rules"`
}

// WebConfig holds the WebUI configuration.
type WebConfig struct {
	Port int `yaml:"port"`
}

// Config is the top-level configuration for the harness.
type Config struct {
	LLM        LLMConfig        `yaml:"llm"`
	Agent      AgentConfig      `yaml:"agent"`
	Governance GovernanceConfig `yaml:"governance"`
	Web        WebConfig        `yaml:"web"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Endpoint:  "https://api.openai.com/v1",
			Model:     "gpt-4o",
			MaxTokens: 4096,
			Timeout:   60 * time.Second,
		},
		Agent: AgentConfig{
			MaxIterations: 10,
			RunTimeout:    300 * time.Second,
		},
		Governance: GovernanceConfig{
			Enabled:       true,
			HITLTimeout:   300 * time.Second,
			ToolTimeout:   30 * time.Second,
			WorkspaceRoot: ".",
			Rules: []string{
				"GIT-001", "GIT-002", "GIT-003",
				"SHELL-001", "SHELL-002",
				"FILE-001",
				"NET-001",
				"PATH-001",
			},
		},
		Web: WebConfig{
			Port: 8080,
		},
	}
}

// Validate checks the configuration for invalid values.
func (c *Config) Validate() error {
	if c.LLM.Endpoint == "" {
		return fmt.Errorf("config: LLM endpoint is required")
	}
	if c.LLM.Model == "" {
		return fmt.Errorf("config: LLM model is required")
	}
	if c.LLM.MaxTokens <= 0 {
		return fmt.Errorf("config: LLM max_tokens must be positive, got %d", c.LLM.MaxTokens)
	}
	if c.LLM.Timeout <= 0 {
		return fmt.Errorf("config: LLM timeout must be positive, got %v", c.LLM.Timeout)
	}
	if c.Agent.MaxIterations <= 0 {
		return fmt.Errorf("config: agent max_iterations must be positive, got %d", c.Agent.MaxIterations)
	}
	if c.Agent.RunTimeout <= 0 {
		return fmt.Errorf("config: agent run_timeout must be positive, got %v", c.Agent.RunTimeout)
	}
	if c.Governance.HITLTimeout <= 0 {
		return fmt.Errorf("config: governance hitl_timeout must be positive, got %v", c.Governance.HITLTimeout)
	}
	if c.Governance.ToolTimeout <= 0 {
		return fmt.Errorf("config: governance tool_timeout must be positive, got %v", c.Governance.ToolTimeout)
	}
	if c.Web.Port <= 0 || c.Web.Port > 65535 {
		return fmt.Errorf("config: web port must be in range 1-65535, got %d", c.Web.Port)
	}
	return nil
}

// ApplyEnvOverrides reads environment variables and overrides config values.
// Environment variables follow the pattern: HARNESS_<SECTION>_<KEY>
// Supports: HARNESS_LLM_ENDPOINT, HARNESS_LLM_MODEL, HARNESS_LLM_MAX_TOKENS,
// HARNESS_LLM_TIMEOUT, HARNESS_AGENT_MAX_ITERATIONS, HARNESS_AGENT_RUN_TIMEOUT,
// HARNESS_GOVERNANCE_ENABLED, HARNESS_GOVERNANCE_HITL_TIMEOUT,
// HARNESS_GOVERNANCE_TOOL_TIMEOUT, HARNESS_GOVERNANCE_WORKSPACE_ROOT,
// HARNESS_WEB_PORT
func (c *Config) ApplyEnvOverrides() {
	// LLM section
	if v := os.Getenv("HARNESS_LLM_ENDPOINT"); v != "" {
		c.LLM.Endpoint = v
	}
	if v := os.Getenv("HARNESS_LLM_MODEL"); v != "" {
		c.LLM.Model = v
	}
	if v := os.Getenv("HARNESS_LLM_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.LLM.MaxTokens = n
		}
	}
	if v := os.Getenv("HARNESS_LLM_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.LLM.Timeout = d
		}
	}

	// Agent section
	if v := os.Getenv("HARNESS_AGENT_MAX_ITERATIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Agent.MaxIterations = n
		}
	}
	if v := os.Getenv("HARNESS_AGENT_RUN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Agent.RunTimeout = d
		}
	}

	// Governance section
	if v := os.Getenv("HARNESS_GOVERNANCE_ENABLED"); v != "" {
		c.Governance.Enabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("HARNESS_GOVERNANCE_HITL_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Governance.HITLTimeout = d
		}
	}
	if v := os.Getenv("HARNESS_GOVERNANCE_TOOL_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Governance.ToolTimeout = d
		}
	}
	if v := os.Getenv("HARNESS_GOVERNANCE_WORKSPACE_ROOT"); v != "" {
		c.Governance.WorkspaceRoot = v
	}

	// Web section
	if v := os.Getenv("HARNESS_WEB_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Web.Port = n
		}
	}
}

// LoadFile reads a YAML config file and returns a Config with defaults + overrides.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file %s: %w", path, err)
	}
	return LoadYAML(data)
}

// LoadYAML parses YAML bytes into a Config, merging with defaults.
func LoadYAML(data []byte) (*Config, error) {
	cfg := DefaultConfig()
	if err := unmarshalYAML(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse YAML: %w", err)
	}
	return cfg, nil
}

// MarshalYAML serializes the Config to YAML bytes.
func (c *Config) MarshalYAML() ([]byte, error) {
	return marshalYAML(c)
}
package config

import "fmt"

// unmarshalYAML parses YAML data into a Config struct.
// Uses a custom minimal YAML parser (no external dependencies).
func unmarshalYAML(data []byte, cfg *Config) error {
	yamlMap, err := parseYAML(data)
	if err != nil {
		return fmt.Errorf("parse YAML: %w", err)
	}
	return applyYAMLToConfig(cfg, yamlMap)
}

// marshalYAML serializes a Config struct to YAML bytes.
// Uses a simple YAML serializer (no external dependencies).
func marshalYAML(cfg *Config) ([]byte, error) {
	return cfg.marshalYAML(), nil
}

// marshalYAML serializes the Config to YAML bytes.
func (c *Config) marshalYAML() []byte {
	var buf []byte
	buf = append(buf, "llm:\n"...)
	buf = append(buf, fmt.Sprintf("  endpoint: %q\n", c.LLM.Endpoint)...)
	buf = append(buf, fmt.Sprintf("  model: %q\n", c.LLM.Model)...)
	buf = append(buf, fmt.Sprintf("  max_tokens: %d\n", c.LLM.MaxTokens)...)
	buf = append(buf, fmt.Sprintf("  timeout: %s\n", c.LLM.Timeout)...)

	buf = append(buf, "agent:\n"...)
	buf = append(buf, fmt.Sprintf("  max_iterations: %d\n", c.Agent.MaxIterations)...)
	buf = append(buf, fmt.Sprintf("  run_timeout: %s\n", c.Agent.RunTimeout)...)

	buf = append(buf, "governance:\n"...)
	buf = append(buf, fmt.Sprintf("  enabled: %t\n", c.Governance.Enabled)...)
	buf = append(buf, fmt.Sprintf("  hitl_timeout: %s\n", c.Governance.HITLTimeout)...)
	buf = append(buf, fmt.Sprintf("  tool_timeout: %s\n", c.Governance.ToolTimeout)...)
	buf = append(buf, fmt.Sprintf("  workspace_root: %q\n", c.Governance.WorkspaceRoot)...)
	buf = append(buf, "  rules:\n"...)
	for _, rule := range c.Governance.Rules {
		buf = append(buf, fmt.Sprintf("    - %s\n", rule)...)
	}

	buf = append(buf, "web:\n"...)
	buf = append(buf, fmt.Sprintf("  port: %d\n", c.Web.Port)...)

	return buf
}
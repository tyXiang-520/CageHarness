package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// yamlParser is a minimal YAML parser for the config structure.
type yamlParser struct {
	lines []string
	pos   int
}

func parseYAML(data []byte) (map[string]any, error) {
	text := string(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")

	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		cleanLines = append(cleanLines, line)
	}

	p := &yamlParser{lines: cleanLines, pos: 0}
	return p.parseBlock(0)
}

func (p *yamlParser) parseBlock(indent int) (map[string]any, error) {
	result := make(map[string]any)

	for p.pos < len(p.lines) {
		line := p.lines[p.pos]
		lineIndent := countIndent(line)

		if lineIndent < indent {
			break
		}

		if lineIndent == indent {
			key, value, isSection, isList, err := p.parseKeyValue(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", p.pos+1, err)
			}

			if isList {
				// This is a key whose value is a list (e.g., "rules:")
				p.pos++
				items, err := p.parseList(indent + 2)
				if err != nil {
					return nil, err
				}
				result[key] = items
			} else if isSection {
				// Section header (e.g., "llm:")
				p.pos++
				if p.pos < len(p.lines) && countIndent(p.lines[p.pos]) > indent {
					child, err := p.parseBlock(indent + 2)
					if err != nil {
						return nil, err
					}
					result[key] = child
				} else {
					result[key] = map[string]any{}
				}
			} else {
				result[key] = value
				p.pos++
			}
		} else {
			p.pos++
		}
	}

	return result, nil
}

func (p *yamlParser) parseList(indent int) ([]string, error) {
	var items []string
	for p.pos < len(p.lines) {
		line := p.lines[p.pos]
		lineIndent := countIndent(line)

		if lineIndent < indent {
			break
		}

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			item := strings.TrimPrefix(trimmed, "- ")
			// Remove quotes if present
			item = strings.Trim(item, "\"'")
			items = append(items, item)
			p.pos++
		} else if trimmed == "-" {
			items = append(items, "")
			p.pos++
		} else {
			break
		}
	}
	return items, nil
}

func (p *yamlParser) parseKeyValue(line string) (key string, value any, isSection bool, isList bool, err error) {
	trimmed := strings.TrimSpace(line)

	colonIdx := strings.Index(trimmed, ":")
	if colonIdx < 0 {
		return "", nil, false, false, fmt.Errorf("expected key-value pair, got %q", trimmed)
	}

	key = strings.TrimSpace(trimmed[:colonIdx])
	rest := strings.TrimSpace(trimmed[colonIdx+1:])

	if rest == "" {
		// Could be a section or a list
		// Check the next line to determine
		nextIdx := p.pos + 1
		if nextIdx < len(p.lines) {
			nextLine := strings.TrimSpace(p.lines[nextIdx])
			if strings.HasPrefix(nextLine, "- ") {
				return key, nil, false, true, nil
			}
		}
		return key, nil, true, false, nil
	}

	value, err = parseValue(rest)
	if err != nil {
		return "", nil, false, false, err
	}

	return key, value, false, false, nil
}

func parseValue(s string) (any, error) {
	// Quoted string
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		return s[1 : len(s)-1], nil
	}

	// Boolean
	if s == "true" || s == "True" || s == "TRUE" {
		return true, nil
	}
	if s == "false" || s == "False" || s == "FALSE" {
		return false, nil
	}

	// Null
	if s == "null" || s == "Null" || s == "NULL" || s == "~" {
		return nil, nil
	}

	// Integer
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}

	// Float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}

	// Duration (e.g., "60s", "300s")
	if isDuration(s) {
		return s, nil
	}

	// Default: string
	return s, nil
}

func isDuration(s string) bool {
	durationSuffixes := []string{"ns", "us", "µs", "ms", "s", "m", "h"}
	for _, suffix := range durationSuffixes {
		if strings.HasSuffix(s, suffix) {
			numPart := strings.TrimSuffix(s, suffix)
			if _, err := strconv.ParseFloat(numPart, 64); err == nil {
				return true
			}
			if _, err := strconv.Atoi(numPart); err == nil {
				return true
			}
		}
	}
	return false
}

func countIndent(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 2
		} else {
			break
		}
	}
	return count
}

func applyYAMLToConfig(cfg *Config, yamlMap map[string]any) error {
	for key, value := range yamlMap {
		switch key {
		case "llm":
			if m, ok := value.(map[string]any); ok {
				if err := applyLLMConfig(&cfg.LLM, m); err != nil {
					return err
				}
			}
		case "agent":
			if m, ok := value.(map[string]any); ok {
				if err := applyAgentConfig(&cfg.Agent, m); err != nil {
					return err
				}
			}
		case "governance":
			if m, ok := value.(map[string]any); ok {
				if err := applyGovernanceConfig(&cfg.Governance, m); err != nil {
					return err
				}
			}
		case "web":
			if m, ok := value.(map[string]any); ok {
				if err := applyWebConfig(&cfg.Web, m); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func applyLLMConfig(llm *LLMConfig, m map[string]any) error {
	for k, v := range m {
		switch k {
		case "endpoint":
			llm.Endpoint = toString(v)
		case "model":
			llm.Model = toString(v)
		case "max_tokens":
			llm.MaxTokens = toInt(v)
		case "timeout":
			llm.Timeout = toDuration(v)
		}
	}
	return nil
}

func applyAgentConfig(agent *AgentConfig, m map[string]any) error {
	for k, v := range m {
		switch k {
		case "max_iterations":
			agent.MaxIterations = toInt(v)
		case "run_timeout":
			agent.RunTimeout = toDuration(v)
		}
	}
	return nil
}

func applyGovernanceConfig(gov *GovernanceConfig, m map[string]any) error {
	for k, v := range m {
		switch k {
		case "enabled":
			gov.Enabled = toBool(v)
		case "hitl_timeout":
			gov.HITLTimeout = toDuration(v)
		case "tool_timeout":
			gov.ToolTimeout = toDuration(v)
		case "workspace_root":
			gov.WorkspaceRoot = toString(v)
		case "rules":
			if items, ok := v.([]string); ok {
				gov.Rules = items
			}
		}
	}
	return nil
}

func applyWebConfig(web *WebConfig, m map[string]any) error {
	for k, v := range m {
		switch k {
		case "port":
			web.Port = toInt(v)
		}
	}
	return nil
}

func toString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
		return 0
	default:
		return 0
	}
}

func toBool(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true" || b == "1"
	default:
		return false
	}
}

func toDuration(v any) time.Duration {
	switch d := v.(type) {
	case time.Duration:
		return d
	case string:
		if dur, err := time.ParseDuration(d); err == nil {
			return dur
		}
		return 0
	case int:
		return time.Duration(d) * time.Second
	default:
		return 0
	}
}
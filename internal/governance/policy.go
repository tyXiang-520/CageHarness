package governance

import (
	"fmt"
	"strings"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// Rule represents a single governance rule.
type Rule struct {
	ID          string
	Description string
	Check       func(action protocol.Action) (bool, string)
}

// PolicyEngine checks actions against a set of governance rules.
type PolicyEngine struct {
	rules []Rule
}

// NewPolicyEngine creates a new PolicyEngine with built-in rules.
func NewPolicyEngine(ruleIDs []string) *PolicyEngine {
	pe := &PolicyEngine{}
	pe.LoadRules(ruleIDs)
	return pe
}

// LoadRules loads the specified rules by ID.
func (pe *PolicyEngine) LoadRules(ruleIDs []string) {
	allRules := builtinRules()
	ruleSet := make(map[string]bool)
	for _, id := range ruleIDs {
		ruleSet[id] = true
	}

	pe.rules = nil
	for _, rule := range allRules {
		if ruleSet[rule.ID] {
			pe.rules = append(pe.rules, rule)
		}
	}
}

// Evaluate checks the action against all loaded rules.
func (pe *PolicyEngine) Evaluate(action protocol.Action) StageResult {
	if len(pe.rules) == 0 {
		return StageResult{
			StageName: "policy",
			Passed:    true,
			Reason:    "no rules loaded",
		}
	}

	var violations []string
	for _, rule := range pe.rules {
		passed, reason := rule.Check(action)
		if !passed {
			violations = append(violations, fmt.Sprintf("%s: %s", rule.ID, reason))
		}
	}

	if len(violations) > 0 {
		return StageResult{
			StageName: "policy",
			Passed:    false,
			Reason:    fmt.Sprintf("violations: %s", strings.Join(violations, "; ")),
		}
	}

	return StageResult{
		StageName: "policy",
		Passed:    true,
	}
}

// builtinRules returns all built-in governance rules.
func builtinRules() []Rule {
	return []Rule{
		{
			ID:          "GIT-001",
			Description: "Prevent force push to main/master",
			Check: func(action protocol.Action) (bool, string) {
				if action.Type != "shell" {
					return true, ""
				}
				cmd, _ := action.Payload["command"].(string)
				if containsSubstring(cmd, "git push") && containsSubstring(cmd, "--force") {
					return false, "force push detected"
				}
				return true, ""
			},
		},
		{
			ID:          "GIT-002",
			Description: "Prevent destructive git operations",
			Check: func(action protocol.Action) (bool, string) {
				if action.Type != "shell" {
					return true, ""
				}
				cmd, _ := action.Payload["command"].(string)
				dangerous := []string{"git reset --hard", "git clean -fd"}
				for _, d := range dangerous {
					if containsSubstring(cmd, d) {
						return false, fmt.Sprintf("destructive git operation: %s", d)
					}
				}
				return true, ""
			},
		},
		{
			ID:          "GIT-003",
			Description: "Prevent commit of sensitive files",
			Check: func(action protocol.Action) (bool, string) {
				if action.Type != "shell" {
					return true, ""
				}
				cmd, _ := action.Payload["command"].(string)
				if containsSubstring(cmd, "git add") {
					sensitive := []string{".env", "id_rsa", "credentials", "secret"}
					for _, s := range sensitive {
						if containsSubstring(cmd, s) {
							return false, fmt.Sprintf("attempting to stage sensitive file: %s", s)
						}
					}
				}
				return true, ""
			},
		},
		{
			ID:          "SHELL-001",
			Description: "Prevent execution of dangerous commands",
			Check: func(action protocol.Action) (bool, string) {
				if action.Type != "shell" {
					return true, ""
				}
				cmd, _ := action.Payload["command"].(string)
				if containsDangerousCommand(cmd) {
					return false, "dangerous command detected"
				}
				return true, ""
			},
		},
		{
			ID:          "SHELL-002",
			Description: "Prevent shell commands with network exposure",
			Check: func(action protocol.Action) (bool, string) {
				if action.Type != "shell" {
					return true, ""
				}
				cmd, _ := action.Payload["command"].(string)
				dangerous := []string{"curl", "wget", "nc ", "telnet"}
				for _, d := range dangerous {
					if containsSubstring(cmd, d) {
						return false, fmt.Sprintf("network command detected: %s", d)
					}
				}
				return true, ""
			},
		},
		{
			ID:          "FILE-001",
			Description: "Prevent file operations outside workspace",
			Check: func(action protocol.Action) (bool, string) {
				if action.Type != "file_read" && action.Type != "file_write" && action.Type != "file_delete" {
					return true, ""
				}
				path, _ := action.Payload["path"].(string)
				if containsDangerousPath(path) {
					return false, fmt.Sprintf("path outside workspace: %s", path)
				}
				return true, ""
			},
		},
		{
			ID:          "NET-001",
			Description: "Prevent network requests to internal hosts",
			Check: func(action protocol.Action) (bool, string) {
				return true, "" // Placeholder: Phase 8+ will implement actual network checks
			},
		},
		{
			ID:          "PATH-001",
			Description: "Prevent path traversal attacks",
			Check: func(action protocol.Action) (bool, string) {
				path, ok := action.Payload["path"].(string)
				if !ok {
					return true, ""
				}
				if containsSubstring(path, "../") || containsSubstring(path, "..\\") {
					return false, "path traversal attempt detected"
				}
				return true, ""
			},
		},
	}
}
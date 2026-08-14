package governance

import (
	"fmt"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// SchemaValidator validates the structure of an Action before it enters the pipeline.
type SchemaValidator struct{}

// NewSchemaValidator creates a new SchemaValidator.
func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{}
}

// Validate checks that the action has all required fields and valid structure.
func (v *SchemaValidator) Validate(action protocol.Action) StageResult {
	if action.ID == "" {
		return StageResult{
			StageName: "schema",
			Passed:    false,
			Reason:    "action ID is empty",
		}
	}
	if action.Type == "" {
		return StageResult{
			StageName: "schema",
			Passed:    false,
			Reason:    "action type is empty",
		}
	}
	if action.Payload == nil {
		return StageResult{
			StageName: "schema",
			Passed:    false,
			Reason:    "action payload is nil",
		}
	}
	return StageResult{
		StageName: "schema",
		Passed:    true,
	}
}

// RiskClassifier assigns a risk level to an action based on its type and payload.
type RiskClassifier struct{}

// NewRiskClassifier creates a new RiskClassifier.
func NewRiskClassifier() *RiskClassifier {
	return &RiskClassifier{}
}

// Classify assigns a risk level based on action type and payload characteristics.
func (c *RiskClassifier) Classify(action protocol.Action) StageResult {
	risk := classifyRisk(action)
	reason := fmt.Sprintf("action type %q classified as %s", action.Type, risk)

	passed := risk != RiskLevelCritical
	// High risk triggers HITL (RequireApproval), not escalation.
	// Escalation is reserved for resource-intensive operations detected by ExecutionController.
	shouldEscalate := false

	return StageResult{
		StageName:      "risk",
		Passed:         passed,
		Reason:         reason,
		RiskLevel:      risk,
		ShouldEscalate: shouldEscalate,
	}
}

// classifyRisk determines the risk level of an action.
func classifyRisk(action protocol.Action) RiskLevel {
	switch action.Type {
	case "shell":
		// Shell commands are high risk by default
		cmd, _ := action.Payload["command"].(string)
		if containsDangerousCommand(cmd) {
			return RiskLevelCritical
		}
		return RiskLevelHigh

	case "file_write":
		// File writes are medium risk
		path, _ := action.Payload["path"].(string)
		if containsDangerousPath(path) {
			return RiskLevelCritical
		}
		return RiskLevelMedium

	case "file_read":
		// File reads are low risk
		return RiskLevelLow

	case "file_delete":
		// File deletes are high risk
		return RiskLevelHigh

	default:
		return RiskLevelMedium
	}
}

// containsDangerousCommand checks for dangerous shell commands.
func containsDangerousCommand(cmd string) bool {
	dangerous := []string{
		"rm -rf /", "mkfs.", "dd if=", ":(){ :|:& };:",
		"> /dev/sda", "chmod 777 /",
	}
	for _, d := range dangerous {
		if containsSubstring(cmd, d) {
			return true
		}
	}
	return false
}

// containsDangerousPath checks for dangerous file paths.
func containsDangerousPath(path string) bool {
	dangerous := []string{
		"/etc/passwd", "/etc/shadow", "/etc/sudoers",
		"~/.ssh", "/root/", "C:\\Windows\\System32\\",
	}
	for _, d := range dangerous {
		if containsSubstring(path, d) {
			return true
		}
	}
	return false
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
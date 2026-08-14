package governance

import (
	"encoding/json"
	"testing"
)

func TestGovernanceDecision_String(t *testing.T) {
	tests := []struct {
		d    GovernanceDecision
		want string
	}{
		{DecisionAllow, "allow"},
		{DecisionDeny, "deny"},
		{DecisionRequireApproval, "require_approval"},
		{DecisionEscalate, "escalate"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.d.String(); got != tt.want {
				t.Errorf("GovernanceDecision.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGovernanceDecision_IsAllowed(t *testing.T) {
	tests := []struct {
		d    GovernanceDecision
		want bool
	}{
		{DecisionAllow, true},
		{DecisionDeny, false},
		{DecisionRequireApproval, false},
		{DecisionEscalate, false},
	}
	for _, tt := range tests {
		t.Run(tt.d.String(), func(t *testing.T) {
			if got := tt.d.IsAllowed(); got != tt.want {
				t.Errorf("GovernanceDecision.IsAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGovernanceDecision_RequiresApproval(t *testing.T) {
	tests := []struct {
		d    GovernanceDecision
		want bool
	}{
		{DecisionAllow, false},
		{DecisionDeny, false},
		{DecisionRequireApproval, true},
		{DecisionEscalate, false},
	}
	for _, tt := range tests {
		t.Run(tt.d.String(), func(t *testing.T) {
			if got := tt.d.RequiresApproval(); got != tt.want {
				t.Errorf("GovernanceDecision.RequiresApproval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuditLogEntry_ZeroValue(t *testing.T) {
	var e AuditLogEntry
	if e.ID != "" {
		t.Errorf("zero value of AuditLogEntry.ID should be empty, got %q", e.ID)
	}
	if e.Decision != DecisionAllow {
		t.Errorf("zero value of AuditLogEntry.Decision should be Allow, got %v", e.Decision)
	}
}

func TestAuditLogEntry_NewEntry(t *testing.T) {
	entry := NewAuditLogEntry("act-1", "shell", DecisionAllow, RiskLevelLow, "agent")
	if entry.ID == "" {
		t.Error("NewAuditLogEntry should generate a non-empty ID")
	}
	if entry.ActionID != "act-1" {
		t.Errorf("ActionID = %q, want %q", entry.ActionID, "act-1")
	}
	if entry.ToolName != "shell" {
		t.Errorf("ToolName = %q, want %q", entry.ToolName, "shell")
	}
	if entry.Decision != DecisionAllow {
		t.Errorf("Decision = %v, want %v", entry.Decision, DecisionAllow)
	}
	if entry.Actor != "agent" {
		t.Errorf("Actor = %q, want %q", entry.Actor, "agent")
	}
	if entry.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestAuditLogEntry_WithDetails(t *testing.T) {
	entry := NewAuditLogEntry("act-2", "file_read", DecisionDeny, RiskLevelMedium, "governance")
	entry.WithDetails(map[string]any{"reason": "risk too high", "risk_level": "critical"})
	if entry.Details["reason"] != "risk too high" {
		t.Errorf("Details[reason] = %v, want %v", entry.Details["reason"], "risk too high")
	}
	if entry.Details["risk_level"] != "critical" {
		t.Errorf("Details[risk_level] = %v, want %v", entry.Details["risk_level"], "critical")
	}
}

func TestAuditLogEntry_DenyEntry(t *testing.T) {
	entry := NewAuditLogEntry("act-3", "file_write", DecisionDeny, RiskLevelHigh, "policy-engine")
	if entry.Decision != DecisionDeny {
		t.Errorf("Decision = %v, want %v", entry.Decision, DecisionDeny)
	}
	if entry.Decision.IsAllowed() {
		t.Error("Deny decision should not be allowed")
	}
}

func TestAuditLogEntry_JSONRoundTrip(t *testing.T) {
	original := NewAuditLogEntry("act-4", "git", DecisionRequireApproval, RiskLevelHigh, "human")
	original.WithDetails(map[string]any{"tool": "shell"})

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded AuditLogEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.ActionID != original.ActionID {
		t.Errorf("ActionID = %q, want %q", decoded.ActionID, original.ActionID)
	}
	if decoded.ToolName != original.ToolName {
		t.Errorf("ToolName = %q, want %q", decoded.ToolName, original.ToolName)
	}
	if decoded.Decision != original.Decision {
		t.Errorf("Decision = %v, want %v", decoded.Decision, original.Decision)
	}
	if decoded.Actor != original.Actor {
		t.Errorf("Actor = %q, want %q", decoded.Actor, original.Actor)
	}
}

func TestAuditLogEntry_RedactSensitive(t *testing.T) {
	entry := NewAuditLogEntry("act-5", "shell", DecisionAllow, RiskLevelLow, "agent")
	entry.WithDetails(map[string]any{
		"api_key": "sk-1234567890abcdef",
		"command": "echo hello",
		"token":   "ghp_testtoken",
	})
	entry.RedactSensitive()

	if entry.Details["api_key"] == "sk-1234567890abcdef" {
		t.Error("api_key should be redacted")
	}
	if entry.Details["token"] == "ghp_testtoken" {
		t.Error("token should be redacted")
	}
	if entry.Details["command"] != "echo hello" {
		t.Errorf("command should not be redacted, got %v", entry.Details["command"])
	}
}

func TestAuditLogEntry_AppendRedact(t *testing.T) {
	entry := NewAuditLogEntry("act-6", "shell", DecisionAllow, RiskLevelLow, "agent")
	entry.AppendRedact("custom_secret")
	entry.WithDetails(map[string]any{"custom_secret": "sensitive_value", "safe": "ok"})
	entry.RedactSensitive()

	if entry.Details["custom_secret"] == "sensitive_value" {
		t.Error("custom_secret should be redacted via AppendRedact")
	}
	if entry.Details["safe"] != "ok" {
		t.Errorf("safe should not be redacted, got %v", entry.Details["safe"])
	}
}

func TestAuditLogEntry_JSONSkipRedact(t *testing.T) {
	// Verify redacted fields are NOT serialized
	entry := NewAuditLogEntry("act-7", "shell", DecisionAllow, RiskLevelLow, "agent")
	entry.WithDetails(map[string]any{"api_key": "secret123"})
	entry.RedactSensitive()

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Check that the redacted field is not present or is redacted
	// This depends on implementation — either the field is removed or replaced
	if details, ok := decoded["details"]; ok {
		if d, ok := details.(map[string]any); ok {
			if v, exists := d["api_key"]; exists && v != "[REDACTED]" {
				t.Errorf("api_key should be redacted in JSON output, got %v", v)
			}
		}
	}
}

func TestAuditLogEntry_RequireApprovalEscalate(t *testing.T) {
	entry := NewAuditLogEntry("act-8", "shell", DecisionEscalate, RiskLevelCritical, "policy-engine")
	if entry.Decision != DecisionEscalate {
		t.Errorf("Decision = %v, want %v", entry.Decision, DecisionEscalate)
	}
	if entry.Decision.RequiresApproval() {
		t.Error("Escalate decision should not require approval")
	}
}
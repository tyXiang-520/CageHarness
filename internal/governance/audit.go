package governance

import (
	"fmt"
	"strings"
	"time"
)

// GovernanceDecision represents the outcome of the governance pipeline.
type GovernanceDecision int

const (
	// DecisionAllow: action is permitted without further approval.
	DecisionAllow GovernanceDecision = iota
	// DecisionDeny: action is rejected.
	DecisionDeny
	// DecisionRequireApproval: action requires human-in-the-loop approval.
	DecisionRequireApproval
	// DecisionEscalate: action requires escalation to a higher authority.
	DecisionEscalate
)

// String returns the human-readable name of the decision.
func (d GovernanceDecision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionDeny:
		return "deny"
	case DecisionRequireApproval:
		return "require_approval"
	case DecisionEscalate:
		return "escalate"
	default:
		return fmt.Sprintf("GovernanceDecision(%d)", int(d))
	}
}

// IsAllowed returns true if the decision permits execution.
func (d GovernanceDecision) IsAllowed() bool {
	return d == DecisionAllow
}

// RequiresApproval returns true if the decision requires human-in-the-loop approval.
func (d GovernanceDecision) RequiresApproval() bool {
	return d == DecisionRequireApproval
}

// AuditLogEntry records a single governance decision for audit trail.
type AuditLogEntry struct {
	ID        string             `json:"id"`
	Timestamp time.Time          `json:"timestamp"`
	ActionID  string             `json:"action_id"`
	ToolName  string             `json:"tool_name"`
	Decision  GovernanceDecision `json:"decision"`
	RiskLevel RiskLevel          `json:"risk_level"`
	Actor     string             `json:"actor"`
	Details   map[string]any     `json:"details,omitempty"`

	// sensitiveKeys holds field names to redact on output.
	// Populated via AppendRedact and consumed by RedactSensitive.
	sensitiveKeys []string `json:"-"`
}

// auditSensitivePrefixes are default field name patterns that should be redacted.
var auditSensitivePrefixes = []string{
	"api_key", "apikey", "token", "secret", "password", "credential",
}

// NewAuditLogEntry creates a new audit log entry with a generated ID.
func NewAuditLogEntry(actionID, toolName string, decision GovernanceDecision, riskLevel RiskLevel, actor string) AuditLogEntry {
	return AuditLogEntry{
		ID:        generateAuditID(),
		Timestamp: time.Now(),
		ActionID:  actionID,
		ToolName:  toolName,
		Decision:  decision,
		RiskLevel: riskLevel,
		Actor:     actor,
		Details:   make(map[string]any),
	}
}

// WithDetails attaches additional context to the audit entry.
func (e *AuditLogEntry) WithDetails(details map[string]any) {
	e.Details = details
}

// AppendRedact adds a field name to the sensitive keys list.
// These fields will be redacted when RedactSensitive is called.
func (e *AuditLogEntry) AppendRedact(key string) {
	e.sensitiveKeys = append(e.sensitiveKeys, key)
}

// RedactSensitive replaces sensitive field values with "[REDACTED]".
// Uses both the default sensitive prefixes and any custom keys added via AppendRedact.
func (e *AuditLogEntry) RedactSensitive() {
	if e.Details == nil {
		return
	}

	// Build redaction set: default prefixes + custom keys
	redactSet := make(map[string]bool)
	for _, key := range e.sensitiveKeys {
		redactSet[key] = true
	}

	for key := range e.Details {
		// Check default prefixes
		for _, prefix := range auditSensitivePrefixes {
			if strings.Contains(strings.ToLower(key), strings.ToLower(prefix)) {
				redactSet[key] = true
				break
			}
		}
	}

	for key := range redactSet {
		if _, exists := e.Details[key]; exists {
			e.Details[key] = "[REDACTED]"
		}
	}
}

// generateAuditID generates a unique audit entry ID.
func generateAuditID() string {
	return fmt.Sprintf("aud-%d", time.Now().UnixNano())
}
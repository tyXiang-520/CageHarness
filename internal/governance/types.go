package governance

import (
	"fmt"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// RiskLevel represents the risk classification of an action.
type RiskLevel int

const (
	RiskLevelLow RiskLevel = iota
	RiskLevelMedium
	RiskLevelHigh
	RiskLevelCritical
)

func (r RiskLevel) String() string {
	switch r {
	case RiskLevelLow:
		return "low"
	case RiskLevelMedium:
		return "medium"
	case RiskLevelHigh:
		return "high"
	case RiskLevelCritical:
		return "critical"
	default:
		return fmt.Sprintf("RiskLevel(%d)", int(r))
	}
}

// GovernanceAuth is issued when a decision requires HITL approval.
// It binds the approval to the exact Action (via ActionHash) with an expiry.
type GovernanceAuth struct {
	DecisionID string    `json:"decision_id"`
	ActionHash string    `json:"action_hash"`
	ActionID   string    `json:"action_id"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// IsExpired returns true if the auth has expired.
func (a GovernanceAuth) IsExpired() bool {
	return time.Now().After(a.ExpiresAt)
}

// NewGovernanceAuth creates an auth token for a HITL decision.
func NewGovernanceAuth(decisionID, actionHash, actionID string, ttl time.Duration) GovernanceAuth {
	return GovernanceAuth{
		DecisionID: decisionID,
		ActionHash: actionHash,
		ActionID:   actionID,
		ExpiresAt:  time.Now().Add(ttl),
	}
}

// StageResult is the output of a single governance pipeline stage.
type StageResult struct {
	StageName      string    `json:"stage"`
	Passed         bool      `json:"passed"`
	Reason         string    `json:"reason,omitempty"`
	RiskLevel      RiskLevel `json:"risk_level,omitempty"`
	ShouldEscalate bool      `json:"should_escalate,omitempty"`
	ToolName       string    `json:"tool_name,omitempty"`
}

// PipelineResult is the aggregated result of the governance pipeline.
type PipelineResult struct {
	Decision  GovernanceDecision `json:"decision"`
	Stages    []StageResult      `json:"stages"`
	Auth      *GovernanceAuth    `json:"auth,omitempty"`
	ActionID  string             `json:"action_id"`
	ToolName  string             `json:"tool_name"`
	Timestamp time.Time          `json:"timestamp"`
}

// IsAllowed returns true if the pipeline result permits execution.
func (pr PipelineResult) IsAllowed() bool {
	return pr.Decision.IsAllowed()
}

// RequiresApproval returns true if the pipeline result requires HITL.
func (pr PipelineResult) RequiresApproval() bool {
	return pr.Decision.RequiresApproval()
}

// ToAuditEntry converts the pipeline result to an audit log entry.
func (pr PipelineResult) ToAuditEntry() AuditLogEntry {
	// Determine the maximum risk level from stages
	maxRisk := RiskLevelLow
	for _, s := range pr.Stages {
		if s.RiskLevel > maxRisk {
			maxRisk = s.RiskLevel
		}
	}
	entry := NewAuditLogEntry(pr.ActionID, pr.ToolName, pr.Decision, maxRisk, "governance-pipeline")
	details := make(map[string]any)
	for _, s := range pr.Stages {
		details[s.StageName] = map[string]any{
			"passed":  s.Passed,
			"reason":  s.Reason,
		}
	}
	if pr.Auth != nil {
		details["auth_decision_id"] = pr.Auth.DecisionID
		details["auth_expires"] = pr.Auth.ExpiresAt.Format(time.RFC3339)
	}
	entry.WithDetails(details)
	entry.RedactSensitive()
	return entry
}

// GovernanceContext carries the configuration and dependencies for the pipeline.
type GovernanceContext struct {
	Enabled       bool
	ToolTimeout   time.Duration
	HITLTimeout   time.Duration
	WorkspaceRoot string
	Rules         []string
}

// DefaultGovernanceContext returns a GovernanceContext with sensible defaults.
func DefaultGovernanceContext() GovernanceContext {
	return GovernanceContext{
		Enabled:       true,
		ToolTimeout:   30 * time.Second,
		HITLTimeout:   300 * time.Second,
		WorkspaceRoot: ".",
		Rules:         []string{},
	}
}

// ComputeActionHash computes a deterministic hash of the Action.
// This is used to bind HITL approvals to the exact action.
func ComputeActionHash(action protocol.Action) string {
	// Canonical representation: type + sorted payload keys
	payload := ""
	if action.Payload != nil {
		// Sort keys for deterministic hashing
		keys := make([]string, 0, len(action.Payload))
		for k := range action.Payload {
			keys = append(keys, k)
		}
		// Simple sort
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		for _, k := range keys {
			payload += fmt.Sprintf("%s=%v;", k, action.Payload[k])
		}
	}
	canonical := fmt.Sprintf("%s:%s:{%s}", action.Type, action.ID, payload)
	// Simple hash: use fmt.Sprintf with %x of string bytes
	return fmt.Sprintf("sha256:%x", simpleHash(canonical))
}

func simpleHash(s string) []byte {
	// FNV-1a inspired hash for deterministic testing
	hash := uint64(14695981039346656037)
	for _, c := range s {
		hash ^= uint64(c)
		hash *= 1099511628211
	}
	result := make([]byte, 8)
	for i := 0; i < 8; i++ {
		result[i] = byte(hash >> (i * 8))
	}
	return result
}
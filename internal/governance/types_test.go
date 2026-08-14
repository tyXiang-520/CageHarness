package governance

import (
	"testing"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

func TestRiskLevel_String(t *testing.T) {
	tests := []struct {
		level RiskLevel
		want  string
	}{
		{RiskLevelLow, "low"},
		{RiskLevelMedium, "medium"},
		{RiskLevelHigh, "high"},
		{RiskLevelCritical, "critical"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("RiskLevel.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGovernanceAuth_IsExpired(t *testing.T) {
	t.Run("not expired", func(t *testing.T) {
		auth := NewGovernanceAuth("dec-1", "hash-1", "act-1", 5*time.Minute)
		if auth.IsExpired() {
			t.Error("auth should not be expired")
		}
	})

	t.Run("expired", func(t *testing.T) {
		auth := NewGovernanceAuth("dec-1", "hash-1", "act-1", -1*time.Second)
		if !auth.IsExpired() {
			t.Error("auth should be expired")
		}
	})
}

func TestGovernanceAuth_BindsToAction(t *testing.T) {
	auth := NewGovernanceAuth("dec-1", "hash-abc", "act-1", 5*time.Minute)
	if auth.DecisionID != "dec-1" {
		t.Errorf("DecisionID = %q, want %q", auth.DecisionID, "dec-1")
	}
	if auth.ActionHash != "hash-abc" {
		t.Errorf("ActionHash = %q, want %q", auth.ActionHash, "hash-abc")
	}
	if auth.ActionID != "act-1" {
		t.Errorf("ActionID = %q, want %q", auth.ActionID, "act-1")
	}
}

func TestComputeActionHash_Deterministic(t *testing.T) {
	action := protocol.NewAction("shell", map[string]any{
		"command": "echo hello",
		"timeout": 30,
	})

	hash1 := ComputeActionHash(action)
	hash2 := ComputeActionHash(action)

	if hash1 != hash2 {
		t.Errorf("hash should be deterministic: %q != %q", hash1, hash2)
	}
}

func TestComputeActionHash_DifferentActions(t *testing.T) {
	a1 := protocol.NewAction("shell", map[string]any{"command": "echo hello"})
	a2 := protocol.NewAction("shell", map[string]any{"command": "rm -rf /"})

	if ComputeActionHash(a1) == ComputeActionHash(a2) {
		t.Error("different actions should have different hashes")
	}
}

func TestComputeActionHash_SamePayloadDifferentOrder(t *testing.T) {
	// Create two actions with same payload but different key order
	a1 := protocol.Action{
		ID:      "act-1",
		Type:    "shell",
		Payload: map[string]any{"command": "echo", "timeout": 30},
	}
	a2 := protocol.Action{
		ID:      "act-1",
		Type:    "shell",
		Payload: map[string]any{"timeout": 30, "command": "echo"},
	}

	if ComputeActionHash(a1) != ComputeActionHash(a2) {
		t.Error("same payload with different key order should produce same hash")
	}
}

func TestPipelineResult_ToAuditEntry(t *testing.T) {
	pr := PipelineResult{
		Decision:  DecisionAllow,
		ActionID:  "act-1",
		Timestamp: time.Now(),
		Stages: []StageResult{
			{StageName: "schema", Passed: true},
			{StageName: "risk", Passed: true, RiskLevel: RiskLevelLow},
		},
	}

	entry := pr.ToAuditEntry()
	if entry.ActionID != "act-1" {
		t.Errorf("ActionID = %q, want %q", entry.ActionID, "act-1")
	}
	if entry.Decision != DecisionAllow {
		t.Errorf("Decision = %v, want %v", entry.Decision, DecisionAllow)
	}
	if entry.Actor != "governance-pipeline" {
		t.Errorf("Actor = %q, want %q", entry.Actor, "governance-pipeline")
	}
}

func TestPipelineResult_WithAuth(t *testing.T) {
	auth := NewGovernanceAuth("dec-1", "hash-1", "act-1", 5*time.Minute)
	pr := PipelineResult{
		Decision:  DecisionRequireApproval,
		ActionID:  "act-1",
		Timestamp: time.Now(),
		Auth:      &auth,
	}

	if !pr.RequiresApproval() {
		t.Error("RequiresApproval should be true")
	}
	if pr.IsAllowed() {
		t.Error("IsAllowed should be false for RequireApproval")
	}

	entry := pr.ToAuditEntry()
	if entry.Decision != DecisionRequireApproval {
		t.Errorf("Decision = %v, want %v", entry.Decision, DecisionRequireApproval)
	}
}

func TestDefaultGovernanceContext(t *testing.T) {
	ctx := DefaultGovernanceContext()
	if !ctx.Enabled {
		t.Error("DefaultGovernanceContext.Enabled should be true")
	}
	if ctx.ToolTimeout <= 0 {
		t.Error("ToolTimeout should be positive")
	}
	if ctx.HITLTimeout <= 0 {
		t.Error("HITLTimeout should be positive")
	}
}
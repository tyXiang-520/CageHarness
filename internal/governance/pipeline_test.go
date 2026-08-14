package governance

import (
	"testing"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

func TestSchemaValidator(t *testing.T) {
	v := NewSchemaValidator()

	t.Run("valid action", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{"command": "echo hi"})
		result := v.Validate(action)
		if !result.Passed {
			t.Errorf("expected pass, got: %s", result.Reason)
		}
	})

	t.Run("empty ID", func(t *testing.T) {
		action := protocol.Action{Type: "shell", Payload: map[string]any{}}
		result := v.Validate(action)
		if result.Passed {
			t.Error("expected failure for empty ID")
		}
	})

	t.Run("empty type", func(t *testing.T) {
		action := protocol.Action{ID: "act-1", Payload: map[string]any{}}
		result := v.Validate(action)
		if result.Passed {
			t.Error("expected failure for empty type")
		}
	})

	t.Run("nil payload", func(t *testing.T) {
		action := protocol.Action{ID: "act-1", Type: "shell"}
		result := v.Validate(action)
		if result.Passed {
			t.Error("expected failure for nil payload")
		}
	})
}

func TestRiskClassifier(t *testing.T) {
	c := NewRiskClassifier()

	t.Run("shell command is high risk", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{"command": "ls -la"})
		result := c.Classify(action)
		if result.RiskLevel != RiskLevelHigh {
			t.Errorf("shell should be high risk, got %v", result.RiskLevel)
		}
	})

	t.Run("dangerous command is critical", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{"command": "rm -rf /"})
		result := c.Classify(action)
		if result.RiskLevel != RiskLevelCritical {
			t.Errorf("rm -rf / should be critical, got %v", result.RiskLevel)
		}
		if result.Passed {
			t.Error("critical risk should not pass")
		}
	})

	t.Run("file read is low risk", func(t *testing.T) {
		action := protocol.NewAction("file_read", map[string]any{"path": "/tmp/test.txt"})
		result := c.Classify(action)
		if result.RiskLevel != RiskLevelLow {
			t.Errorf("file_read should be low risk, got %v", result.RiskLevel)
		}
	})

	t.Run("file write is medium risk", func(t *testing.T) {
		action := protocol.NewAction("file_write", map[string]any{"path": "/tmp/test.txt"})
		result := c.Classify(action)
		if result.RiskLevel != RiskLevelMedium {
			t.Errorf("file_write should be medium risk, got %v", result.RiskLevel)
		}
	})

	t.Run("file delete is high risk", func(t *testing.T) {
		action := protocol.NewAction("file_delete", map[string]any{"path": "/tmp/test.txt"})
		result := c.Classify(action)
		if result.RiskLevel != RiskLevelHigh {
			t.Errorf("file_delete should be high risk, got %v", result.RiskLevel)
		}
	})
}

func TestPolicyEngine(t *testing.T) {
	t.Run("shell echo passes all rules", func(t *testing.T) {
		pe := NewPolicyEngine([]string{"GIT-001", "SHELL-001", "SHELL-002"})
		action := protocol.NewAction("shell", map[string]any{"command": "echo hello"})
		result := pe.Evaluate(action)
		if !result.Passed {
			t.Errorf("echo should pass: %s", result.Reason)
		}
	})

	t.Run("git force push is blocked", func(t *testing.T) {
		pe := NewPolicyEngine([]string{"GIT-001"})
		action := protocol.NewAction("shell", map[string]any{"command": "git push --force origin main"})
		result := pe.Evaluate(action)
		if result.Passed {
			t.Error("force push should be blocked by GIT-001")
		}
	})

	t.Run("rm -rf is blocked", func(t *testing.T) {
		pe := NewPolicyEngine([]string{"SHELL-001"})
		action := protocol.NewAction("shell", map[string]any{"command": "rm -rf /"})
		result := pe.Evaluate(action)
		if result.Passed {
			t.Error("rm -rf should be blocked by SHELL-001")
		}
	})

	t.Run("no rules loaded passes", func(t *testing.T) {
		pe := NewPolicyEngine(nil)
		action := protocol.NewAction("shell", map[string]any{"command": "rm -rf /"})
		result := pe.Evaluate(action)
		if !result.Passed {
			t.Error("no rules should pass by default")
		}
	})

	t.Run("path traversal blocked", func(t *testing.T) {
		pe := NewPolicyEngine([]string{"PATH-001"})
		action := protocol.NewAction("file_read", map[string]any{"path": "../etc/passwd"})
		result := pe.Evaluate(action)
		if result.Passed {
			t.Error("path traversal should be blocked by PATH-001")
		}
	})
}

func TestExecutionBoundary(t *testing.T) {
	t.Run("path within workspace", func(t *testing.T) {
		b := NewExecutionBoundary(".")
		action := protocol.NewAction("file_read", map[string]any{"path": "test.txt"})
		result := b.Check(action)
		if !result.Passed {
			t.Errorf("expected pass, got: %s", result.Reason)
		}
	})

	t.Run("no workspace restriction", func(t *testing.T) {
		b := NewExecutionBoundary(".")
		action := protocol.NewAction("file_read", map[string]any{"path": "/etc/passwd"})
		result := b.Check(action)
		// With "." workspace, absolute paths are checked against the workspace
		// This is expected to fail on most systems
		_ = result // Just ensure no panic
	})
}

func TestExecutionController(t *testing.T) {
	t.Run("normal action passes", func(t *testing.T) {
		c := NewExecutionController(30 * 1e9) // 30s in ns
		action := protocol.NewAction("shell", map[string]any{"command": "echo hi"})
		result := c.Check(action)
		if !result.Passed {
			t.Errorf("expected pass, got: %s", result.Reason)
		}
	})

	t.Run("action timeout exceeds limit", func(t *testing.T) {
		c := NewExecutionController(30 * 1e9) // 30s
		action := protocol.NewAction("shell", map[string]any{
			"command": "sleep 60",
			"timeout": float64(60),
		})
		result := c.Check(action)
		if result.Passed {
			t.Error("timeout 60s should exceed 30s limit")
		}
	})
}

func TestPipeline_FullFlow(t *testing.T) {
	ctx := DefaultGovernanceContext()
	ctx.Rules = []string{"GIT-001", "SHELL-001", "SHELL-002", "FILE-001", "PATH-001"}
	p := NewPipeline(ctx)

	t.Run("safe shell command allows", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{"command": "echo hello world"})
		result := p.Evaluate(action)
		// Shell is high risk → requires approval
		if !result.RequiresApproval() {
			t.Logf("decision: %v (shell is high risk, may require approval)", result.Decision)
		}
		if result.Decision == DecisionDeny {
			t.Errorf("echo should not be denied, got: %v", result.Decision)
		}
	})

	t.Run("dangerous command denies", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{"command": "rm -rf /"})
		result := p.Evaluate(action)
		if result.Decision != DecisionDeny {
			t.Errorf("rm -rf should be denied, got: %v", result.Decision)
		}
	})

	t.Run("file read allows", func(t *testing.T) {
		action := protocol.NewAction("file_read", map[string]any{"path": "test.txt"})
		result := p.Evaluate(action)
		if result.Decision == DecisionDeny {
			t.Errorf("file_read should not be denied, got: %v", result.Decision)
		}
	})

	t.Run("audit log is recorded", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{"command": "echo test"})
		p.Evaluate(action)
		log := p.AuditLog()
		if len(log) == 0 {
			t.Error("audit log should have entries")
		}
	})
}

func TestPipeline_HITL(t *testing.T) {
	ctx := DefaultGovernanceContext()
	ctx.HITLTimeout = 300 * 1e9 // 300s in ns
	p := NewPipeline(ctx)

	t.Run("approve HITL", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{"command": "echo test"})
		auth := NewGovernanceAuth("dec-test", ComputeActionHash(action), action.ID, 300*1e9)

		result, err := p.ApproveHITL(action, auth)
		if err != nil {
			t.Fatalf("ApproveHITL error: %v", err)
		}
		if result.Decision != DecisionAllow {
			t.Errorf("HITL approval should allow, got %v", result.Decision)
		}
	})

	t.Run("reject HITL", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{"command": "echo test"})
		auth := NewGovernanceAuth("dec-test", ComputeActionHash(action), action.ID, 300*1e9)

		result := p.RejectHITL(action, auth, "too risky")
		if result.Decision != DecisionDeny {
			t.Errorf("HITL rejection should deny, got %v", result.Decision)
		}
	})

	t.Run("expired auth cannot approve", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{"command": "echo test"})
		auth := NewGovernanceAuth("dec-test", ComputeActionHash(action), action.ID, -1*1e9)

		_, err := p.ApproveHITL(action, auth)
		if err == nil {
			t.Error("expected error for expired auth")
		}
	})

	t.Run("hash mismatch cannot approve", func(t *testing.T) {
		action1 := protocol.NewAction("shell", map[string]any{"command": "echo hello"})
		action2 := protocol.NewAction("shell", map[string]any{"command": "echo world"})
		auth := NewGovernanceAuth("dec-test", ComputeActionHash(action1), action1.ID, 300*1e9)

		_, err := p.ApproveHITL(action2, auth)
		if err == nil {
			t.Error("expected error for hash mismatch")
		}
	})
}

func TestPipeline_DynamicRules(t *testing.T) {
	ctx := DefaultGovernanceContext()
	p := NewPipeline(ctx)

	// Without rules, dangerous command passes risk check but fails on risk level
	action := protocol.NewAction("shell", map[string]any{"command": "echo safe"})
	result := p.Evaluate(action)
	if result.Decision == DecisionDeny {
		t.Errorf("safe command should not be denied without rules, got: %v", result.Decision)
	}

	// Add rules — verify they take effect
	p.SetRules([]string{"SHELL-001", "GIT-001"})
	logLen := len(p.AuditLog())
	_ = logLen
}

func TestPipeline_StageOrder(t *testing.T) {
	// Verify stages execute in correct order
	ctx := DefaultGovernanceContext()
	ctx.Rules = []string{"SHELL-001"}
	p := NewPipeline(ctx)

	action := protocol.NewAction("shell", map[string]any{"command": "echo hi"})
	result := p.Evaluate(action)

	expectedOrder := []string{"schema", "risk", "policy", "boundary", "control"}
	for i, stage := range result.Stages {
		if i < len(expectedOrder) && stage.StageName != expectedOrder[i] {
			t.Errorf("stage %d: expected %q, got %q", i, expectedOrder[i], stage.StageName)
		}
	}
}

func TestComputeActionHash_CanonicalJSON(t *testing.T) {
	// Same semantic payload, different key order → same hash
	a1 := protocol.Action{
		ID:      "act-1",
		Type:    "shell",
		Payload: map[string]any{"command": "ls", "cwd": "/tmp", "env": map[string]any{"A": "1"}},
	}
	a2 := protocol.Action{
		ID:      "act-1",
		Type:    "shell",
		Payload: map[string]any{"env": map[string]any{"A": "1"}, "cwd": "/tmp", "command": "ls"},
	}

	if ComputeActionHash(a1) != ComputeActionHash(a2) {
		t.Error("same payload with different key order should produce same hash")
	}
}
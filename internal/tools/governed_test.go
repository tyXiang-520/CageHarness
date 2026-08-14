package tools

import (
	"testing"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

func TestGovernedTool_ExecutesThroughGovernance(t *testing.T) {
	// Verify the architecture invariant: Agent NEVER directly calls Tool.Execute()
	// The Governance layer must intercept all tool invocations.

	var executed bool
	mt := &mockTool{
		name: "test",
		execute: func(action protocol.Action) (ToolResult, error) {
			executed = true
			return NewSuccessResult(action.ID, "ok", 0), nil
		},
	}

	// Simulate governance interception: the Governance layer calls Execute
	gt := NewGovernedTool(mt)
	action := protocol.NewAction("test", nil)

	result, err := gt.Execute(action)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if !executed {
		t.Error("underlying tool should have been executed")
	}
}

func TestGovernedTool_ValidateDelegates(t *testing.T) {
	mt := &mockTool{
		name: "test",
		validate: func(action protocol.Action) error {
			return nil
		},
	}

	gt := NewGovernedTool(mt)
	action := protocol.NewAction("test", nil)
	if err := gt.Validate(action); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestGovernedTool_ChainOfResponsibility(t *testing.T) {
	// Verify that GovernedTool implements Tool (so it can be used transparently)
	var _ Tool = (*GovernedTool)(nil)

	// Verify the chain: Agent → Governance → GovernedTool → actual Tool
	mt := &mockTool{name: "chain", desc: "chain test"}
	gt := NewGovernedTool(mt)

	if gt.Name() != "chain" {
		t.Errorf("Name() = %q, want %q", gt.Name(), "chain")
	}
	if gt.Description() != "chain test" {
		t.Errorf("Description() = %q, want %q", gt.Description(), "chain test")
	}
}

func TestGovernedTool_NilTool(t *testing.T) {
	// Nil tool should be handled gracefully
	gt := NewGovernedTool(nil)
	action := protocol.NewAction("test", nil)
	result, err := gt.Execute(action)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Success {
		t.Error("expected failure for nil underlying tool")
	}
}
package tools

import (
	"fmt"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// GovernedTool wraps a Tool to ensure it is executed through the Governance pipeline.
// This is the only way a tool should be invoked — the Agent Loop must never call
// Tool.Execute() directly. Instead, the Governance layer creates a GovernedTool,
// runs its decision pipeline, and then calls GovernedTool.Execute().
//
// Architecture Invariant (SPEC §5):
//
//	Agent → Action → Governance → GovernedTool → Tool.Execute() → ToolResult
//
// The Agent does NOT:
//
//	Agent → Action → Tool.Execute()  ← FORBIDDEN
type GovernedTool struct {
	inner Tool
}

// NewGovernedTool creates a governance-wrapped tool.
// The inner tool may be nil (for testing/dry-run scenarios); Execute will return
// an error result in that case.
func NewGovernedTool(inner Tool) *GovernedTool {
	return &GovernedTool{inner: inner}
}

// Name delegates to the inner tool, or returns "governed(nil)" if inner is nil.
func (g *GovernedTool) Name() string {
	if g.inner == nil {
		return "governed(nil)"
	}
	return g.inner.Name()
}

// Description delegates to the inner tool.
func (g *GovernedTool) Description() string {
	if g.inner == nil {
		return ""
	}
	return g.inner.Description()
}

// Execute runs the tool through the governance wrapper.
// In Phase 6, this will be the point where the Governance pipeline
// (Schema Validation → Risk Classification → Policy Engine → Execution Boundary → Execution Control)
// runs before delegating to the inner tool.
func (g *GovernedTool) Execute(action protocol.Action) (protocol.ToolResult, error) {
	if g.inner == nil {
		return protocol.NewErrorResult(action.ID, "governed tool: inner tool is nil", 0), nil
	}

	// Placeholder: Phase 6 will insert governance checks here
	// before delegating to the inner tool.

	return g.inner.Execute(action)
}

// Validate delegates to the inner tool, or returns an error if inner is nil.
func (g *GovernedTool) Validate(action protocol.Action) error {
	if g.inner == nil {
		return fmt.Errorf("governed tool: inner tool is nil")
	}
	return g.inner.Validate(action)
}

// Inner returns the underlying tool. This is used by the Governance layer
// to inspect tool metadata without executing.
func (g *GovernedTool) Inner() Tool {
	return g.inner
}
package governance

import (
	"fmt"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// Pipeline orchestrates the 5-layer governance pipeline.
// Architecture: Schema → Risk → Policy → Boundary → Control → Decision
type Pipeline struct {
	validator  *SchemaValidator
	classifier *RiskClassifier
	policy     *PolicyEngine
	boundary   *ExecutionBoundary
	controller *ExecutionController
	context    GovernanceContext
	auditLog   []AuditLogEntry
}

// NewPipeline creates a new governance Pipeline.
func NewPipeline(ctx GovernanceContext) *Pipeline {
	return &Pipeline{
		validator:  NewSchemaValidator(),
		classifier: NewRiskClassifier(),
		policy:     NewPolicyEngine(ctx.Rules),
		boundary:   NewExecutionBoundary(ctx.WorkspaceRoot),
		controller: NewExecutionController(ctx.ToolTimeout),
		context:    ctx,
	}
}

// Evaluate runs the full governance pipeline on an action.
// Returns a PipelineResult with the decision and stage results.
func (p *Pipeline) Evaluate(action protocol.Action) PipelineResult {
	actionHash := ComputeActionHash(action)
	result := PipelineResult{
		ActionID:  action.ID,
		ToolName:  action.Type,
		Timestamp: time.Now(),
	}

	// Stage 1: Schema Validation
	schemaResult := p.validator.Validate(action)
	result.Stages = append(result.Stages, schemaResult)
	if !schemaResult.Passed {
		result.Decision = DecisionDeny
		p.recordAudit(result)
		return result
	}

	// Stage 2: Risk Classification
	riskResult := p.classifier.Classify(action)
	result.Stages = append(result.Stages, riskResult)
	if !riskResult.Passed {
		result.Decision = DecisionDeny
		p.recordAudit(result)
		return result
	}

	// Stage 3: Policy Engine
	policyResult := p.policy.Evaluate(action)
	result.Stages = append(result.Stages, policyResult)
	if !policyResult.Passed {
		result.Decision = DecisionDeny
		p.recordAudit(result)
		return result
	}

	// Stage 4: Execution Boundary
	boundaryResult := p.boundary.Check(action)
	result.Stages = append(result.Stages, boundaryResult)
	if !boundaryResult.Passed {
		result.Decision = DecisionDeny
		p.recordAudit(result)
		return result
	}

	// Stage 5: Execution Control
	controlResult := p.controller.Check(action)
	result.Stages = append(result.Stages, controlResult)
	if !controlResult.Passed {
		if controlResult.ShouldEscalate {
			result.Decision = DecisionEscalate
		} else {
			result.Decision = DecisionDeny
		}
		p.recordAudit(result)
		return result
	}

	// Decision: aggregate results
	result.Decision = p.makeDecision(result.Stages)

	// If RequireApproval, generate GovernanceAuth
	if result.Decision.RequiresApproval() {
		auth := NewGovernanceAuth(
			fmt.Sprintf("dec-%d", time.Now().UnixNano()),
			actionHash,
			action.ID,
			p.context.HITLTimeout,
		)
		result.Auth = &auth
	}

	p.recordAudit(result)
	return result
}

// makeDecision determines the final decision based on stage results.
func (p *Pipeline) makeDecision(stages []StageResult) GovernanceDecision {
	// Check for escalation
	for _, s := range stages {
		if s.ShouldEscalate {
			return DecisionEscalate
		}
	}

	// Check risk levels
	for _, s := range stages {
		if s.RiskLevel >= RiskLevelHigh {
			// High risk requires HITL approval
			return DecisionRequireApproval
		}
	}

	return DecisionAllow
}

// ApproveHITL approves a previously pending HITL action.
// The auth must match the original action hash.
func (p *Pipeline) ApproveHITL(action protocol.Action, auth GovernanceAuth) (PipelineResult, error) {
	if auth.IsExpired() {
		return PipelineResult{}, fmt.Errorf("HITL auth expired at %s", auth.ExpiresAt.Format(time.RFC3339))
	}

	currentHash := ComputeActionHash(action)
	if currentHash != auth.ActionHash {
		return PipelineResult{}, fmt.Errorf("action hash mismatch: HITL auth is for %s, got %s", auth.ActionHash, currentHash)
	}

	result := PipelineResult{
		Decision:  DecisionAllow,
		ActionID:  action.ID,
		ToolName:  action.Type,
		Timestamp: time.Now(),
		Auth:      &auth,
		Stages: []StageResult{
			{StageName: "hitl", Passed: true, Reason: "approved by human"},
		},
	}
	p.recordAudit(result)
	return result, nil
}

// RejectHITL rejects a previously pending HITL action.
func (p *Pipeline) RejectHITL(action protocol.Action, auth GovernanceAuth, reason string) PipelineResult {
	result := PipelineResult{
		Decision:  DecisionDeny,
		ActionID:  action.ID,
		ToolName:  action.Type,
		Timestamp: time.Now(),
		Auth:      &auth,
		Stages: []StageResult{
			{StageName: "hitl", Passed: false, Reason: reason},
		},
	}
	p.recordAudit(result)
	return result
}

// AuditLog returns all recorded audit entries.
func (p *Pipeline) AuditLog() []AuditLogEntry {
	return p.auditLog
}

// recordAudit stores a pipeline result as an audit log entry.
func (p *Pipeline) recordAudit(result PipelineResult) {
	entry := result.ToAuditEntry()
	p.auditLog = append(p.auditLog, entry)
}

// SetRules updates the policy engine rules at runtime.
func (p *Pipeline) SetRules(ruleIDs []string) {
	p.policy.LoadRules(ruleIDs)
}
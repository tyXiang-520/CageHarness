package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/agent"
	"github.com/tyXiang-520/CageHarness/internal/feedback"
	"github.com/tyXiang-520/CageHarness/internal/governance"
	"github.com/tyXiang-520/CageHarness/internal/llm"
	"github.com/tyXiang-520/CageHarness/internal/memory"
	"github.com/tyXiang-520/CageHarness/internal/protocol"
	"github.com/tyXiang-520/CageHarness/internal/tools"
)

// LoopConfig holds configuration for the Agent Loop.
type LoopConfig struct {
	// MaxIterations is the maximum number of tool-call iterations before the loop terminates.
	MaxIterations int
	// SystemPrompt is the initial system message that sets the Agent's behavior.
	SystemPrompt string
	// ToolTimeout is passed to the Governance pipeline for execution control.
	ToolTimeout time.Duration
	// HITLTimeout is the validity duration for HITL approval tokens.
	HITLTimeout time.Duration
}

// DefaultLoopConfig returns a sensible default configuration.
func DefaultLoopConfig() LoopConfig {
	return LoopConfig{
		MaxIterations: 10,
		SystemPrompt:  "You are a helpful coding assistant.",
		ToolTimeout:   30 * time.Second,
		HITLTimeout:   300 * time.Second,
	}
}

// StateTransition records a state change in the Agent Loop.
type StateTransition struct {
	From      agent.AgentState
	To        agent.AgentState
	Timestamp time.Time
}

// HITLHandler is called when the Governance pipeline requires human approval.
// Return DecisionAllow to approve, DecisionDeny to reject.
type HITLHandler func(action protocol.Action, auth governance.GovernanceAuth) governance.GovernanceDecision

// AgentLoop orchestrates the observe-think-decide-act cycle.
//
// Architecture: AgentLoop is the runtime orchestrator. It imports all packages
// (agent, governance, tools, llm, protocol) and wires them together at runtime.
// No individual package imports another — they all communicate through protocol types.
//
//	AgentLoop
//	  ├─→ llm.Generate(messages)          // Think
//	  ├─→ governance.Pipeline.Evaluate()  // Decide (is this allowed?)
//	  ├─→ tools.Registry.Execute()        // Act
//	  └─→ agent.NewObservation()          // Observe
type AgentLoop struct {
	state            agent.AgentState
	llm              llm.Provider
	governance       *governance.Pipeline
	tools            *tools.Registry
	messages         []llm.Message
	config           LoopConfig
	iterations       int
	stateTransitions []StateTransition
	hitlHandler      HITLHandler
	feedback         *feedback.FeedbackProcessor
	memoryStore      *memory.FileStore
	memoryRetriever  *memory.Retriever
}

// NewAgentLoop creates a new AgentLoop.
func NewAgentLoop(llmProvider llm.Provider, gov *governance.Pipeline, toolReg *tools.Registry, config LoopConfig) *AgentLoop {
	return &AgentLoop{
		state:      agent.AgentStateIdle,
		llm:        llmProvider,
		governance: gov,
		tools:      toolReg,
		config:     config,
		feedback:   feedback.NewFeedbackProcessor(),
	}
}

// SetMemory configures the memory store and retriever for the Agent Loop.
// When set, relevant memories are injected into the system prompt on each Run() invocation.
func (a *AgentLoop) SetMemory(store *memory.FileStore) {
	a.memoryStore = store
	a.memoryRetriever = memory.NewRetriever(store)
}

// Run executes the main agent loop for a given task.
// Returns the final text response or an error.
func (a *AgentLoop) Run(ctx context.Context, task string) (string, error) {
	// Build the system prompt, injecting relevant memories if available
	systemPrompt := a.buildSystemPrompt(task)

	// Initialize the conversation
	a.messages = []llm.Message{
		llm.NewSystemMessage(systemPrompt),
		llm.NewMessage(llm.RoleUser, task),
	}

	for a.iterations < a.config.MaxIterations {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Phase: Think — call the LLM
		a.transition(agent.AgentStateThinking)

		response, err := a.llm.Generate(ctx, a.messages)
		if err != nil {
			a.transition(agent.AgentStateError)
			return "", fmt.Errorf("llm generate: %w", err)
		}

		// Append the assistant's response to the conversation
		a.messages = append(a.messages, response.Message)

		switch response.FinishReason {
		case llm.FinishReasonStop:
			// LLM produced a final answer — we're done
			a.transition(agent.AgentStateTerminated)
			return response.Message.Content, nil

		case llm.FinishReasonToolCalls:
			// LLM wants to call tools — execute them through governance
			for _, tc := range response.Message.ToolCalls {
				execErr := a.executeToolCall(ctx, tc)
				if execErr != nil {
					// If the error is due to HITL being required, return the error
					// so the caller can handle approval
					if isHITLError(execErr) {
						return "", execErr
					}
					// Otherwise, append the error as a tool message and continue
					errMsg := fmt.Sprintf(`{"error": %q}`, execErr.Error())
					a.messages = append(a.messages, llm.NewToolMessage(tc.ID, errMsg))
				}
			}
			a.iterations++

		case llm.FinishReasonError:
			a.transition(agent.AgentStateError)
			return "", fmt.Errorf("llm returned error: %s", response.Message.Content)

		default:
			a.transition(agent.AgentStateError)
			return "", fmt.Errorf("unexpected finish reason: %s", response.FinishReason)
		}
	}

	a.transition(agent.AgentStateError)
	return "", fmt.Errorf("max iterations (%d) exceeded", a.config.MaxIterations)
}

// buildSystemPrompt constructs the system prompt, injecting relevant memories if available.
func (a *AgentLoop) buildSystemPrompt(task string) string {
	prompt := a.config.SystemPrompt

	// Inject relevant memories if memory is configured
	if a.memoryRetriever != nil {
		memories := a.memoryRetriever.Retrieve(task, 3)
		if len(memories) > 0 {
			var sb strings.Builder
			sb.WriteString(prompt)
			sb.WriteString("\n\nRelevant context from memory:\n")
			for _, mem := range memories {
				sb.WriteString("- ")
				sb.WriteString(mem.Content)
				sb.WriteString("\n")
			}
			prompt = sb.String()
		}
	}

	return prompt
}

// executeToolCall processes a single tool call through the governance pipeline.
func (a *AgentLoop) executeToolCall(ctx context.Context, tc llm.ToolCall) error {
	// Parse the tool arguments from JSON
	payload, err := parseToolArguments(tc.Function.Arguments)
	if err != nil {
		return fmt.Errorf("parse tool arguments for %s: %w", tc.Function.Name, err)
	}

	// Create the action
	action := protocol.NewAction(tc.Function.Name, payload)

	// Phase: Decide — run through governance
	a.transition(agent.AgentStateAwaitingApproval)

	decision := a.governance.Evaluate(action)

	switch decision.Decision {
	case governance.DecisionDeny:
		// Governance blocked the action
		return fmt.Errorf("governance denied action %s: %s", action.ID, a.governanceDenyReason(decision))

	case governance.DecisionEscalate:
		return fmt.Errorf("governance escalated action %s", action.ID)

	case governance.DecisionRequireApproval:
		// HITL required
		if a.hitlHandler == nil {
			return &hitlError{
				ActionID: action.ID,
				Auth:     *decision.Auth,
				Reason:   "HITL approval required but no handler configured",
			}
		}
		hitlDecision := a.hitlHandler(action, *decision.Auth)
		if hitlDecision == governance.DecisionDeny {
			return fmt.Errorf("HITL rejected action %s", action.ID)
		}
		// Approved — fall through to execution
	}

	// Phase: Act — execute the tool
	a.transition(agent.AgentStateExecuting)

	tool, ok := a.tools.Get(tc.Function.Name)
	if !ok {
		return fmt.Errorf("tool %q not found in registry", tc.Function.Name)
	}

	// Validate the action before executing
	if err := tool.Validate(action); err != nil {
		return fmt.Errorf("tool %s validation failed: %w", tc.Function.Name, err)
	}

	result, execErr := tool.Execute(action)
	if execErr != nil {
		return fmt.Errorf("tool %s execution failed: %w", tc.Function.Name, execErr)
	}

	// Phase: Observe — record the result
	a.transition(agent.AgentStateObserving)

	// Attach result to action
	action.WithResult(&result)

	// Process through feedback to get structured observation
	obs := a.feedback.Process(tc.Function.Name, result)
	obsMsg := obs.FormatForLLM()

	// Append the observation as a tool message
	a.messages = append(a.messages, llm.NewToolMessage(tc.ID, obsMsg))

	return nil
}

// SetHITLHandler configures the Human-in-the-Loop approval callback.
// When set, actions requiring approval will be passed to this handler.
// If not set, actions requiring HITL will cause an error.
func (a *AgentLoop) SetHITLHandler(handler HITLHandler) {
	a.hitlHandler = handler
}

// Messages returns the current conversation history.
func (a *AgentLoop) Messages() []llm.Message {
	result := make([]llm.Message, len(a.messages))
	copy(result, a.messages)
	return result
}

// State returns the current Agent state.
func (a *AgentLoop) State() agent.AgentState {
	return a.state
}

// StateTransitions returns the recorded state transitions.
func (a *AgentLoop) StateTransitions() []StateTransition {
	result := make([]StateTransition, len(a.stateTransitions))
	copy(result, a.stateTransitions)
	return result
}

// Iterations returns the number of tool-call iterations executed.
func (a *AgentLoop) Iterations() int {
	return a.iterations
}

// transition attempts a state transition and records it.
// If the transition is invalid, it logs the error but does not panic.
func (a *AgentLoop) transition(next agent.AgentState) {
	newState, err := a.state.TransitionTo(next)
	if err != nil {
		// Invalid transition — record the attempt but keep current state
		a.stateTransitions = append(a.stateTransitions, StateTransition{
			From:      a.state,
			To:        next,
			Timestamp: time.Now(),
		})
		return
	}
	a.stateTransitions = append(a.stateTransitions, StateTransition{
		From:      a.state,
		To:        newState,
		Timestamp: time.Now(),
	})
	a.state = newState
}

// parseToolArguments parses a JSON string into a map[string]any payload.
func parseToolArguments(args string) (map[string]any, error) {
	if args == "" || args == "{}" {
		return map[string]any{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(args), &payload); err != nil {
		return nil, fmt.Errorf("invalid tool arguments JSON: %w", err)
	}
	return payload, nil
}

// hitlError represents a HITL-required error that the caller can handle.
type hitlError struct {
	ActionID string
	Auth     governance.GovernanceAuth
	Reason   string
}

func (e *hitlError) Error() string {
	return fmt.Sprintf("HITL required for action %s: %s", e.ActionID, e.Reason)
}

func isHITLError(err error) bool {
	_, ok := err.(*hitlError)
	return ok
}

// governanceDenyReason extracts the reason for denial from the pipeline stages.
func (a *AgentLoop) governanceDenyReason(result governance.PipelineResult) string {
	for _, s := range result.Stages {
		if !s.Passed {
			return s.Reason
		}
	}
	return result.Decision.String()
}
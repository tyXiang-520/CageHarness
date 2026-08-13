package agent

import "fmt"

// AgentState represents the state of the Agent in its decision loop.
// Zero value is AgentStateIdle, which is the initial state.
type AgentState int

const (
	// AgentStateIdle is the initial state, waiting for a task.
	AgentStateIdle AgentState = iota
	// AgentStateThinking: LLM is generating a response.
	AgentStateThinking
	// AgentStateAwaitingApproval: waiting for human-in-the-loop approval.
	AgentStateAwaitingApproval
	// AgentStateExecuting: executing a tool action.
	AgentStateExecuting
	// AgentStateObserving: processing the result of a tool execution.
	AgentStateObserving
	// AgentStateError: an unrecoverable error occurred.
	AgentStateError
	// AgentStateTerminated: the agent loop has finished.
	AgentStateTerminated
)

// String returns the human-readable name of the state.
func (s AgentState) String() string {
	switch s {
	case AgentStateIdle:
		return "idle"
	case AgentStateThinking:
		return "thinking"
	case AgentStateAwaitingApproval:
		return "awaiting_approval"
	case AgentStateExecuting:
		return "executing"
	case AgentStateObserving:
		return "observing"
	case AgentStateError:
		return "error"
	case AgentStateTerminated:
		return "terminated"
	default:
		return fmt.Sprintf("AgentState(%d)", int(s))
	}
}

// IsTerminal returns true if the state is a terminal state (no further transitions possible).
func (s AgentState) IsTerminal() bool {
	return s == AgentStateTerminated
}

// transitionTable defines valid state transitions.
// Map key: current state, value: set of allowed next states.
var transitionTable = map[AgentState]map[AgentState]bool{
	AgentStateIdle:             {AgentStateThinking: true},
	AgentStateThinking:         {AgentStateExecuting: true, AgentStateAwaitingApproval: true, AgentStateError: true, AgentStateTerminated: true},
	AgentStateAwaitingApproval: {AgentStateExecuting: true, AgentStateError: true, AgentStateTerminated: true},
	AgentStateExecuting:        {AgentStateObserving: true, AgentStateError: true, AgentStateTerminated: true},
	AgentStateObserving:        {AgentStateThinking: true, AgentStateError: true, AgentStateTerminated: true},
	AgentStateError:            {AgentStateTerminated: true},
	AgentStateTerminated:       {},
}

// CanTransitionTo returns true if the transition from s to next is valid.
func (s AgentState) CanTransitionTo(next AgentState) bool {
	if allowed, ok := transitionTable[s]; ok {
		return allowed[next]
	}
	return false
}

// TransitionTo attempts to transition to the next state.
// Returns an error if the transition is invalid.
func (s AgentState) TransitionTo(next AgentState) (AgentState, error) {
	if !s.CanTransitionTo(next) {
		return s, fmt.Errorf("invalid state transition: %s → %s", s, next)
	}
	return next, nil
}
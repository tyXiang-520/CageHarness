package agent

import (
	"fmt"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/tools"
)

// ActionStatus represents the lifecycle status of an Action.
type ActionStatus int

const (
	// ActionStatusPending: action created but not yet executed.
	ActionStatusPending ActionStatus = iota
	// ActionStatusRunning: action is currently being executed.
	ActionStatusRunning
	// ActionStatusCompleted: action executed successfully.
	ActionStatusCompleted
	// ActionStatusFailed: action execution failed.
	ActionStatusFailed
	// ActionStatusCancelled: action was cancelled before completion.
	ActionStatusCancelled
)

// String returns the human-readable name of the action status.
func (s ActionStatus) String() string {
	switch s {
	case ActionStatusPending:
		return "pending"
	case ActionStatusRunning:
		return "running"
	case ActionStatusCompleted:
		return "completed"
	case ActionStatusFailed:
		return "failed"
	case ActionStatusCancelled:
		return "cancelled"
	default:
		return fmt.Sprintf("ActionStatus(%d)", int(s))
	}
}

// actionStatusTransitions defines valid status transitions.
var actionStatusTransitions = map[ActionStatus]map[ActionStatus]bool{
	ActionStatusPending:   {ActionStatusRunning: true, ActionStatusCancelled: true},
	ActionStatusRunning:   {ActionStatusCompleted: true, ActionStatusFailed: true, ActionStatusCancelled: true},
	ActionStatusCompleted: {},
	ActionStatusFailed:    {},
	ActionStatusCancelled: {},
}

// Action is the protocol between Agent and Tools.
// Owned by the agent package to avoid circular dependencies (governance depends on tools, tools would depend on governance via Action).
type Action struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload,omitempty"`
	Status    ActionStatus   `json:"status"`
	Result    *tools.ToolResult `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// NewAction creates a new Action with a generated ID and Pending status.
func NewAction(actionType string, payload map[string]any) Action {
	return Action{
		ID:        generateActionID(),
		Type:      actionType,
		Payload:   payload,
		Status:    ActionStatusPending,
		Timestamp: time.Now(),
	}
}

// ActionFromResult creates an Action from an existing ToolResult.
// Used when reconstituting actions from audit logs or observations.
func ActionFromResult(id, actionType string, result tools.ToolResult) Action {
	status := ActionStatusCompleted
	if !result.Success {
		status = ActionStatusFailed
	}
	return Action{
		ID:        id,
		Type:      actionType,
		Status:    status,
		Result:    &result,
		Timestamp: result.Timestamp,
	}
}

// SetStatus transitions the action to a new status.
// Returns an error if the transition is invalid.
func (a *Action) SetStatus(newStatus ActionStatus) error {
	if !actionStatusTransitions[a.Status][newStatus] {
		return fmt.Errorf("invalid action status transition: %s → %s", a.Status, newStatus)
	}
	a.Status = newStatus
	return nil
}

// WithResult attaches a ToolResult to the Action and updates its status.
func (a *Action) WithResult(result *tools.ToolResult) {
	a.Result = result
	if result.Success {
		a.Status = ActionStatusCompleted
	} else {
		a.Status = ActionStatusFailed
		a.Error = result.Error
	}
}

// generateActionID generates a unique action ID.
// In Phase 1 this is a simple timestamp-based ID.
// In later phases this will use a more robust ID generation mechanism.
func generateActionID() string {
	return fmt.Sprintf("act-%d", time.Now().UnixNano())
}
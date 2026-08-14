package agent

import (
	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// Action is the protocol between Agent and Tools.
// This is a type alias for protocol.Action to avoid circular imports.
type Action = protocol.Action

// ActionStatus represents the lifecycle status of an Action.
type ActionStatus = protocol.ActionStatus

// Re-exported constants for backward compatibility.
const (
	ActionStatusPending   = protocol.ActionStatusPending
	ActionStatusRunning   = protocol.ActionStatusRunning
	ActionStatusCompleted = protocol.ActionStatusCompleted
	ActionStatusFailed    = protocol.ActionStatusFailed
	ActionStatusCancelled = protocol.ActionStatusCancelled
)

// Re-exported functions.
var (
	NewAction        = protocol.NewAction
	ActionFromResult = protocol.ActionFromResult
)
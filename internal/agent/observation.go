package agent

import (
	"time"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// Observation represents what the Agent observes after executing an Action.
// It wraps the ToolResult with the ActionID and a timestamp.
type Observation struct {
	ActionID  string              `json:"action_id"`
	Result    protocol.ToolResult `json:"result"`
	Timestamp time.Time           `json:"timestamp"`
}

// NewObservation creates an Observation from an Action and its result.
func NewObservation(action Action) Observation {
	result := protocol.ToolResult{}
	if action.Result != nil {
		result = *action.Result
	}
	return Observation{
		ActionID:  action.ID,
		Result:    result,
		Timestamp: time.Now(),
	}
}

// IsError returns true if the observation represents a failed execution.
func (o Observation) IsError() bool {
	return !o.Result.Success
}
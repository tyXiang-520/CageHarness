package tools

import (
	"encoding/json"
	"time"
)

// ToolResult represents the result of a tool execution.
// It is part of the Action protocol between Agent and Tools.
type ToolResult struct {
	ActionID  string        `json:"action_id"`
	Success   bool          `json:"success"`
	Data      any           `json:"data,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration_ns"`
	Timestamp time.Time     `json:"timestamp"`
}

// NewSuccessResult creates a ToolResult for a successful execution.
func NewSuccessResult(actionID string, data any, duration time.Duration) ToolResult {
	return ToolResult{
		ActionID:  actionID,
		Success:   true,
		Data:      data,
		Duration:  duration,
		Timestamp: time.Now(),
	}
}

// NewErrorResult creates a ToolResult for a failed execution.
func NewErrorResult(actionID, errMsg string, duration time.Duration) ToolResult {
	return ToolResult{
		ActionID:  actionID,
		Success:   false,
		Error:     errMsg,
		Duration:  duration,
		Timestamp: time.Now(),
	}
}

// IsZero returns true if the ToolResult is its zero value.
func (r ToolResult) IsZero() bool {
	return r.ActionID == "" && !r.Success && r.Data == nil && r.Error == "" && r.Duration == 0 && r.Timestamp.IsZero()
}

// MarshalJSON implements json.Marshaler for ToolResult.
// time.Duration is marshaled as nanoseconds for precision.
func (r ToolResult) MarshalJSON() ([]byte, error) {
	type Alias ToolResult
	return json.Marshal(&struct {
		Duration int64 `json:"duration_ns"`
		*Alias
	}{
		Duration: int64(r.Duration),
		Alias:    (*Alias)(&r),
	})
}

// UnmarshalJSON implements json.Unmarshaler for ToolResult.
func (r *ToolResult) UnmarshalJSON(data []byte) error {
	type Alias ToolResult
	aux := &struct {
		Duration int64 `json:"duration_ns"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	r.Duration = time.Duration(aux.Duration)
	return nil
}
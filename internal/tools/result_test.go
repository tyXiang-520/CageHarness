package tools

import (
	"testing"
	"time"
)

func TestToolResult_ZeroValue(t *testing.T) {
	var r ToolResult
	if r.Success != false {
		t.Error("zero value of ToolResult.Success should be false")
	}
	if r.Error != "" {
		t.Errorf("zero value of ToolResult.Error should be empty, got %q", r.Error)
	}
	if r.Data != nil {
		t.Error("zero value of ToolResult.Data should be nil")
	}
}

func TestToolResult_SuccessResult(t *testing.T) {
	r := NewSuccessResult("my-action", "hello world", 150*time.Millisecond)
	if r.ActionID != "my-action" {
		t.Errorf("ActionID = %q, want %q", r.ActionID, "my-action")
	}
	if !r.Success {
		t.Error("Success should be true")
	}
	if r.Data != "hello world" {
		t.Errorf("Data = %v, want %v", r.Data, "hello world")
	}
	if r.Error != "" {
		t.Errorf("Error should be empty, got %q", r.Error)
	}
	if r.Duration != 150*time.Millisecond {
		t.Errorf("Duration = %v, want %v", r.Duration, 150*time.Millisecond)
	}
	if r.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestToolResult_ErrorResult(t *testing.T) {
	r := NewErrorResult("my-action", "permission denied", 50*time.Millisecond)
	if r.ActionID != "my-action" {
		t.Errorf("ActionID = %q, want %q", r.ActionID, "my-action")
	}
	if r.Success {
		t.Error("Success should be false")
	}
	if r.Data != nil {
		t.Error("Data should be nil for error result")
	}
	if r.Error != "permission denied" {
		t.Errorf("Error = %q, want %q", r.Error, "permission denied")
	}
	if r.Duration != 50*time.Millisecond {
		t.Errorf("Duration = %v, want %v", r.Duration, 50*time.Millisecond)
	}
}

func TestToolResult_JSONRoundTrip(t *testing.T) {
	original := NewSuccessResult("action-1", map[string]int{"count": 42}, 200*time.Millisecond)
	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var decoded ToolResult
	if err := decoded.UnmarshalJSON(data); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if decoded.ActionID != original.ActionID {
		t.Errorf("ActionID = %q, want %q", decoded.ActionID, original.ActionID)
	}
	if decoded.Success != original.Success {
		t.Errorf("Success = %v, want %v", decoded.Success, original.Success)
	}
	if decoded.Error != original.Error {
		t.Errorf("Error = %q, want %q", decoded.Error, original.Error)
	}
}

func TestToolResult_ErrorResultJSON(t *testing.T) {
	original := NewErrorResult("action-2", "timeout", 5*time.Second)
	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var decoded ToolResult
	if err := decoded.UnmarshalJSON(data); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if decoded.Success {
		t.Error("Success should be false for error result")
	}
	if decoded.Error != "timeout" {
		t.Errorf("Error = %q, want %q", decoded.Error, "timeout")
	}
}

func TestToolResult_IsZero(t *testing.T) {
	var r ToolResult
	if !r.IsZero() {
		t.Error("zero ToolResult should return true for IsZero")
	}

	r = NewSuccessResult("a", "data", 0)
	if r.IsZero() {
		t.Error("non-zero ToolResult should return false for IsZero")
	}
}
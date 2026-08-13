package agent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/tools"
)

func TestObservation_ZeroValue(t *testing.T) {
	var o Observation
	if o.ActionID != "" {
		t.Errorf("zero value of Observation.ActionID should be empty, got %q", o.ActionID)
	}
	if o.Timestamp.IsZero() == false {
		t.Error("zero value of Observation.Timestamp should be zero")
	}
}

func TestObservation_NewObservation(t *testing.T) {
	action := NewAction("shell", map[string]any{"cmd": "echo hello"})
	r := tools.NewSuccessResult(action.ID, "hello", 10*time.Millisecond)
	action.WithResult(&r)

	obs := NewObservation(action)
	if obs.ActionID != action.ID {
		t.Errorf("ActionID = %q, want %q", obs.ActionID, action.ID)
	}
	if obs.Result.Success != true {
		t.Error("Result.Success should be true")
	}
	if obs.Result.Data != "hello" {
		t.Errorf("Result.Data = %v, want %v", obs.Result.Data, "hello")
	}
	if obs.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestObservation_NewObservationFromError(t *testing.T) {
	action := NewAction("shell", map[string]any{"cmd": "invalid"})
	r := tools.NewErrorResult(action.ID, "command not found", 5*time.Millisecond)
	action.WithResult(&r)

	obs := NewObservation(action)
	if obs.ActionID != action.ID {
		t.Errorf("ActionID = %q, want %q", obs.ActionID, action.ID)
	}
	if obs.Result.Success {
		t.Error("Result.Success should be false")
	}
	if obs.Result.Error != "command not found" {
		t.Errorf("Result.Error = %q, want %q", obs.Result.Error, "command not found")
	}
}

func TestObservation_JSONRoundTrip(t *testing.T) {
	action := NewAction("file_read", map[string]any{"path": "/tmp/test.txt"})
	r := tools.NewSuccessResult(action.ID, "file content", 3*time.Millisecond)
	action.WithResult(&r)

	original := NewObservation(action)
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Observation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.ActionID != original.ActionID {
		t.Errorf("ActionID = %q, want %q", decoded.ActionID, original.ActionID)
	}
	if decoded.Result.Success != original.Result.Success {
		t.Errorf("Result.Success = %v, want %v", decoded.Result.Success, original.Result.Success)
	}
}

func TestObservation_IsError(t *testing.T) {
	t.Run("error observation", func(t *testing.T) {
		r := tools.NewErrorResult("act-1", "fail", 0)
		o := Observation{ActionID: "act-1", Result: r, Timestamp: time.Now()}
		if !o.IsError() {
			t.Error("IsError should be true for error result")
		}
	})

	t.Run("success observation", func(t *testing.T) {
		r := tools.NewSuccessResult("act-2", "ok", 0)
		o := Observation{ActionID: "act-2", Result: r, Timestamp: time.Now()}
		if o.IsError() {
			t.Error("IsError should be false for success result")
		}
	})
}
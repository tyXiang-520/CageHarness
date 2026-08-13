package agent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/tools"
)

func TestAction_ZeroValue(t *testing.T) {
	var a Action
	if a.ID != "" {
		t.Errorf("zero value of Action.ID should be empty, got %q", a.ID)
	}
	if a.Status != ActionStatusPending {
		t.Errorf("zero value of Action.Status should be Pending, got %v", a.Status)
	}
	if a.Result != nil {
		t.Error("zero value of Action.Result should be nil")
	}
}

func TestAction_NewAction(t *testing.T) {
	payload := map[string]any{"command": "ls -la"}
	a := NewAction("shell", payload)
	if a.ID == "" {
		t.Error("NewAction should generate a non-empty ID")
	}
	if a.Type != "shell" {
		t.Errorf("Type = %q, want %q", a.Type, "shell")
	}
	if a.Payload["command"] != "ls -la" {
		t.Errorf("Payload = %v, want %v", a.Payload, payload)
	}
	if a.Status != ActionStatusPending {
		t.Errorf("Status should be Pending, got %v", a.Status)
	}
	if a.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestAction_StatusTransitions(t *testing.T) {
	a := NewAction("test", nil)

	// Pending → Running
	if err := a.SetStatus(ActionStatusRunning); err != nil {
		t.Fatalf("Pending→Running: %v", err)
	}

	// Running → Completed
	if err := a.SetStatus(ActionStatusCompleted); err != nil {
		t.Fatalf("Running→Completed: %v", err)
	}

	// Completed → Failed (invalid)
	if err := a.SetStatus(ActionStatusFailed); err == nil {
		t.Error("expected error for Completed→Failed transition")
	}
}

func TestAction_WithResult(t *testing.T) {
	a := NewAction("shell", map[string]any{"cmd": "echo hi"})
	r := tools.NewSuccessResult(a.ID, "hi", 10*time.Millisecond)
	a.WithResult(&r)

	if a.Result == nil {
		t.Fatal("Result should be non-nil after WithResult")
	}
	if !a.Result.Success {
		t.Error("Result.Success should be true")
	}
	if a.Result.Data != "hi" {
		t.Errorf("Result.Data = %v, want %v", a.Result.Data, "hi")
	}
}

func TestAction_JSONRoundTrip(t *testing.T) {
	payload := map[string]any{"key": "value"}
	original := NewAction("file_read", payload)
	original.SetStatus(ActionStatusRunning)

	r := tools.NewSuccessResult(original.ID, "content", 5*time.Millisecond)
	original.WithResult(&r)

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Action
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, original.Type)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status = %v, want %v", decoded.Status, original.Status)
	}
	if decoded.Result == nil || decoded.Result.Success != original.Result.Success {
		t.Error("Result round-trip failed")
	}
}

func TestAction_ActionStatusString(t *testing.T) {
	tests := []struct {
		status ActionStatus
		want   string
	}{
		{ActionStatusPending, "pending"},
		{ActionStatusRunning, "running"},
		{ActionStatusCompleted, "completed"},
		{ActionStatusFailed, "failed"},
		{ActionStatusCancelled, "cancelled"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("ActionStatus.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAction_FromResult(t *testing.T) {
	r := tools.NewSuccessResult("act-1", "output", 100*time.Millisecond)
	a := ActionFromResult("act-1", "shell", r)

	if a.ID != "act-1" {
		t.Errorf("ID = %q, want %q", a.ID, "act-1")
	}
	if a.Type != "shell" {
		t.Errorf("Type = %q, want %q", a.Type, "shell")
	}
	if a.Result == nil {
		t.Fatal("Result should be non-nil")
	}
	if !a.Result.Success {
		t.Error("Result.Success should be true")
	}
	if a.Status != ActionStatusCompleted {
		t.Errorf("Status should be Completed, got %v", a.Status)
	}
}
package agent

import (
	"encoding/json"
	"testing"
)

func TestAgentState_ZeroValue(t *testing.T) {
	var s AgentState
	if s != AgentStateIdle {
		t.Errorf("zero value of AgentState should be Idle, got %v", s)
	}
}

func TestAgentState_String(t *testing.T) {
	tests := []struct {
		state AgentState
		want  string
	}{
		{AgentStateIdle, "idle"},
		{AgentStateThinking, "thinking"},
		{AgentStateAwaitingApproval, "awaiting_approval"},
		{AgentStateExecuting, "executing"},
		{AgentStateObserving, "observing"},
		{AgentStateError, "error"},
		{AgentStateTerminated, "terminated"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("AgentState.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAgentState_IsTerminal(t *testing.T) {
	tests := []struct {
		state AgentState
		want  bool
	}{
		{AgentStateIdle, false},
		{AgentStateThinking, false},
		{AgentStateAwaitingApproval, false},
		{AgentStateExecuting, false},
		{AgentStateObserving, false},
		{AgentStateError, false},
		{AgentStateTerminated, true},
	}
	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			if got := tt.state.IsTerminal(); got != tt.want {
				t.Errorf("AgentState.IsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgentState_JSONRoundTrip(t *testing.T) {
	original := AgentStateThinking
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded AgentState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("JSON round-trip = %v, want %v", decoded, original)
	}
}

func TestAgentState_ValidTransitions(t *testing.T) {
	tests := []struct {
		name  string
		from  AgentState
		to    AgentState
		valid bool
	}{
		{"idle→thinking", AgentStateIdle, AgentStateThinking, true},
		{"thinking→executing", AgentStateThinking, AgentStateExecuting, true},
		{"executing→observing", AgentStateExecuting, AgentStateObserving, true},
		{"observing→thinking", AgentStateObserving, AgentStateThinking, true},
		{"thinking→awaiting_approval", AgentStateThinking, AgentStateAwaitingApproval, true},
		{"awaiting_approval→executing", AgentStateAwaitingApproval, AgentStateExecuting, true},
		{"any→error", AgentStateExecuting, AgentStateError, true},
		{"any→terminated", AgentStateObserving, AgentStateTerminated, true},
		{"idle→executing", AgentStateIdle, AgentStateExecuting, false},
		{"idle→terminated", AgentStateIdle, AgentStateTerminated, false},
		{"terminated→thinking", AgentStateTerminated, AgentStateThinking, false},
		{"error→executing", AgentStateError, AgentStateExecuting, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			if got != tt.valid {
				t.Errorf("CanTransitionTo(%v) = %v, want %v", tt.to, got, tt.valid)
			}
		})
	}
}

func TestAgentState_TransitionTo(t *testing.T) {
	t.Run("valid transition", func(t *testing.T) {
		next, err := AgentStateIdle.TransitionTo(AgentStateThinking)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if next != AgentStateThinking {
			t.Errorf("got %v, want %v", next, AgentStateThinking)
		}
	})

	t.Run("invalid transition", func(t *testing.T) {
		_, err := AgentStateIdle.TransitionTo(AgentStateExecuting)
		if err == nil {
			t.Fatal("expected error for invalid transition, got nil")
		}
	})
}
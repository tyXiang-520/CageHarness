package llm

import (
	"testing"
)

func TestMessage_NewMessage(t *testing.T) {
	msg := NewMessage(RoleUser, "hello")
	if msg.Role != RoleUser {
		t.Errorf("Role = %v, want %v", msg.Role, RoleUser)
	}
	if msg.Content != "hello" {
		t.Errorf("Content = %q, want %q", msg.Content, "hello")
	}
}

func TestMessage_NewSystemMessage(t *testing.T) {
	msg := NewSystemMessage("You are a helpful assistant.")
	if msg.Role != RoleSystem {
		t.Errorf("Role = %v, want %v", msg.Role, RoleSystem)
	}
	if msg.Content != "You are a helpful assistant." {
		t.Errorf("Content = %q, want %q", msg.Content, "You are a helpful assistant.")
	}
}

func TestMessage_NewToolMessage(t *testing.T) {
	msg := NewToolMessage("tool-123", "result data")
	if msg.Role != RoleTool {
		t.Errorf("Role = %v, want %v", msg.Role, RoleTool)
	}
	if msg.Content != "result data" {
		t.Errorf("Content = %q, want %q", msg.Content, "result data")
	}
	if msg.ToolCallID != "tool-123" {
		t.Errorf("ToolCallID = %q, want %q", msg.ToolCallID, "tool-123")
	}
}

func TestRole_String(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleSystem, "system"},
		{RoleUser, "user"},
		{RoleAssistant, "assistant"},
		{RoleTool, "tool"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.role.String(); got != tt.want {
				t.Errorf("Role.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMessage_WithToolCall(t *testing.T) {
	msg := NewMessage(RoleAssistant, "")
	msg.WithToolCall("call-1", "read_file", `{"path": "/tmp/test.txt"}`)
	if msg.ToolCalls == nil {
		t.Fatal("ToolCalls should not be nil")
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "call-1" {
		t.Errorf("ToolCall.ID = %q, want %q", tc.ID, "call-1")
	}
	if tc.Function.Name != "read_file" {
		t.Errorf("ToolCall.Function.Name = %q, want %q", tc.Function.Name, "read_file")
	}
	if tc.Function.Arguments != `{"path": "/tmp/test.txt"}` {
		t.Errorf("ToolCall.Function.Arguments = %q, want %q", tc.Function.Arguments, `{"path": "/tmp/test.txt"}`)
	}
}

func TestToolCallFunction_String(t *testing.T) {
	tcf := ToolCallFunction{Name: "shell", Arguments: `{"cmd": "ls"}`}
	if tcf.String() != "shell(`{\"cmd\": \"ls\"}`)" {
		t.Errorf("ToolCallFunction.String() = %q, want %q", tcf.String(), "shell(`{\"cmd\": \"ls\"}`)")
	}
}

func TestFinishReason_String(t *testing.T) {
	tests := []struct {
		fr   FinishReason
		want string
	}{
		{FinishReasonStop, "stop"},
		{FinishReasonToolCalls, "tool_calls"},
		{FinishReasonLength, "length"},
		{FinishReasonError, "error"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.fr.String(); got != tt.want {
				t.Errorf("FinishReason.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
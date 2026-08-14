package tools

import (
	"testing"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

func TestShellTool_Name(t *testing.T) {
	st := NewShellTool(ShellConfig{})
	if st.Name() != "shell" {
		t.Errorf("Name() = %q, want %q", st.Name(), "shell")
	}
}

func TestShellTool_Description(t *testing.T) {
	st := NewShellTool(ShellConfig{})
	if st.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestShellTool_Validate(t *testing.T) {
	st := NewShellTool(ShellConfig{})

	t.Run("valid action", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{
			"command": "echo hello",
		})
		if err := st.Validate(action); err != nil {
			t.Errorf("unexpected validation error: %v", err)
		}
	})

	t.Run("missing command", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{})
		if err := st.Validate(action); err == nil {
			t.Error("expected error for missing command")
		}
	})

	t.Run("empty command", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{
			"command": "",
		})
		if err := st.Validate(action); err == nil {
			t.Error("expected error for empty command")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{
			"command": 123,
		})
		if err := st.Validate(action); err == nil {
			t.Error("expected error for non-string command")
		}
	})
}

func TestShellTool_Execute(t *testing.T) {
	st := NewShellTool(ShellConfig{})

	t.Run("echo command", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{
			"command": "echo hello world",
		})
		result, err := st.Execute(action)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if !result.Success {
			t.Error("expected success for echo command")
		}
		if result.Data == nil {
			t.Error("Data should not be nil")
		}
	})

	t.Run("failed command", func(t *testing.T) {
		action := protocol.NewAction("shell", map[string]any{
			"command": "nonexistent_command_xyz",
		})
		result, err := st.Execute(action)
		// Execute should not return error; failure is in the result
		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}
		if result.Success {
			t.Error("expected failure for nonexistent command")
		}
	})

	t.Run("command with timeout", func(t *testing.T) {
		st := NewShellTool(ShellConfig{
			Timeout: 100 * time.Millisecond,
		})
		action := protocol.NewAction("shell", map[string]any{
			"command": "sleep 10",
		})
		result, err := st.Execute(action)
		if err != nil {
			t.Fatalf("Execute() unexpected error: %v", err)
		}
		if result.Success {
			t.Error("expected failure for timed out command")
		}
	})
}

func TestShellTool_ImplementsInterface(t *testing.T) {
	var _ Tool = (*ShellTool)(nil)
}
package tools

import (
	"testing"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// mockTool implements Tool for testing the interface contract.
type mockTool struct {
	name    string
	desc    string
	execute func(action protocol.Action) (ToolResult, error)
	validate func(action protocol.Action) error
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return m.desc }
func (m *mockTool) Execute(action protocol.Action) (ToolResult, error) {
	if m.execute != nil {
		return m.execute(action)
	}
	return NewSuccessResult(action.ID, nil, 0), nil
}
func (m *mockTool) Validate(action protocol.Action) error {
	if m.validate != nil {
		return m.validate(action)
	}
	return nil
}

func TestToolInterface(t *testing.T) {
	t.Run("implements Tool interface", func(t *testing.T) {
		var _ Tool = (*mockTool)(nil) // compile-time check
		mt := &mockTool{name: "test", desc: "a test tool"}
		if mt.Name() != "test" {
			t.Errorf("Name() = %q, want %q", mt.Name(), "test")
		}
		if mt.Description() != "a test tool" {
			t.Errorf("Description() = %q, want %q", mt.Description(), "a test tool")
		}
	})

	t.Run("execute returns ToolResult", func(t *testing.T) {
		mt := &mockTool{name: "test"}
		action := protocol.NewAction("test", nil)
		result, err := mt.Execute(action)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if result.ActionID != action.ID {
			t.Errorf("result.ActionID = %q, want %q", result.ActionID, action.ID)
		}
		if !result.Success {
			t.Error("result.Success should be true")
		}
	})

	t.Run("ToolResult belongs to tools package", func(t *testing.T) {
		mt := &mockTool{name: "test"}
		action := protocol.NewAction("test", nil)
		result, _ := mt.Execute(action)
		_ = result.ActionID // ToolResult field access - compile check
	})
}

func TestToolRegistry(t *testing.T) {
	t.Run("register and get", func(t *testing.T) {
		reg := NewRegistry()
		mt := &mockTool{name: "echo", desc: "echoes input"}
		reg.Register(mt)

		got, ok := reg.Get("echo")
		if !ok {
			t.Fatal("expected tool 'echo' to be registered")
		}
		if got.Name() != "echo" {
			t.Errorf("Name() = %q, want %q", got.Name(), "echo")
		}
	})

	t.Run("get non-existent", func(t *testing.T) {
		reg := NewRegistry()
		_, ok := reg.Get("nonexistent")
		if ok {
			t.Error("expected false for non-existent tool")
		}
	})

	t.Run("list all tools", func(t *testing.T) {
		reg := NewRegistry()
		reg.Register(&mockTool{name: "a"})
		reg.Register(&mockTool{name: "b"})

		tools := reg.List()
		if len(tools) != 2 {
			t.Fatalf("expected 2 tools, got %d", len(tools))
		}
	})

	t.Run("register duplicate", func(t *testing.T) {
		reg := NewRegistry()
		reg.Register(&mockTool{name: "dup"})
		err := reg.Register(&mockTool{name: "dup"})
		if err == nil {
			t.Error("expected error for duplicate registration")
		}
	})
}
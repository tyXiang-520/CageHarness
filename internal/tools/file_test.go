package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

func TestFileTool_Name(t *testing.T) {
	ft := NewFileTool(FileConfig{WorkspaceRoot: t.TempDir()})
	if ft.Name() != "file" {
		t.Errorf("Name() = %q, want %q", ft.Name(), "file")
	}
}

func TestFileTool_Description(t *testing.T) {
	ft := NewFileTool(FileConfig{WorkspaceRoot: t.TempDir()})
	if ft.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestFileTool_Validate(t *testing.T) {
	workspace := t.TempDir()
	ft := NewFileTool(FileConfig{WorkspaceRoot: workspace})

	t.Run("valid read action", func(t *testing.T) {
		action := protocol.NewAction("file_read", map[string]any{
			"path": filepath.Join(workspace, "test.txt"),
		})
		if err := ft.Validate(action); err != nil {
			t.Errorf("unexpected validation error: %v", err)
		}
	})

	t.Run("valid write action", func(t *testing.T) {
		action := protocol.NewAction("file_write", map[string]any{
			"path":    filepath.Join(workspace, "test.txt"),
			"content": "hello",
		})
		if err := ft.Validate(action); err != nil {
			t.Errorf("unexpected validation error: %v", err)
		}
	})

	t.Run("missing path", func(t *testing.T) {
		action := protocol.NewAction("file_read", map[string]any{})
		if err := ft.Validate(action); err == nil {
			t.Error("expected error for missing path")
		}
	})

	t.Run("path outside workspace", func(t *testing.T) {
		action := protocol.NewAction("file_read", map[string]any{
			"path": filepath.Join(workspace, "..", "outside.txt"),
		})
		if err := ft.Validate(action); err == nil {
			t.Error("expected error for path outside workspace")
		}
	})

	t.Run("path traversal attempt", func(t *testing.T) {
		action := protocol.NewAction("file_read", map[string]any{
			"path": filepath.Join(workspace, "a", "..", "..", "..", "outside.txt"),
		})
		if err := ft.Validate(action); err == nil {
			t.Error("expected error for path traversal")
		}
	})
}

func TestFileTool_ReadWrite(t *testing.T) {
	// Create a temp directory as workspace
	workspace := t.TempDir()
	ft := NewFileTool(FileConfig{WorkspaceRoot: workspace})

	t.Run("write file", func(t *testing.T) {
		writePath := filepath.Join(workspace, "hello.txt")
		action := protocol.NewAction("file_write", map[string]any{
			"path":    writePath,
			"content": "hello world",
		})
		result, err := ft.Execute(action)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if !result.Success {
			t.Errorf("write failed: %s", result.Error)
		}
		// Verify file exists on disk
		data, err := os.ReadFile(writePath)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		if string(data) != "hello world" {
			t.Errorf("file content = %q, want %q", string(data), "hello world")
		}
	})

	t.Run("read file", func(t *testing.T) {
		readPath := filepath.Join(workspace, "hello.txt")
		action := protocol.NewAction("file_read", map[string]any{
			"path": readPath,
		})
		result, err := ft.Execute(action)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if !result.Success {
			t.Errorf("read failed: %s", result.Error)
		}
		if result.Data != "hello world" {
			t.Errorf("read Data = %v, want %q", result.Data, "hello world")
		}
	})

	t.Run("read non-existent file", func(t *testing.T) {
		nonExistPath := filepath.Join(workspace, "nope.txt")
		action := protocol.NewAction("file_read", map[string]any{
			"path": nonExistPath,
		})
		result, err := ft.Execute(action)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if result.Success {
			t.Error("expected failure for non-existent file")
		}
	})
}

func TestFileTool_ImplementsInterface(t *testing.T) {
	var _ Tool = (*FileTool)(nil)
}
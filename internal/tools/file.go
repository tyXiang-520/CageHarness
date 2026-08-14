package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// FileConfig holds configuration for the FileTool.
type FileConfig struct {
	// WorkspaceRoot is the root directory that file operations are restricted to.
	WorkspaceRoot string
}

// FileTool reads and writes files within a workspace root.
// It implements the Tool interface.
type FileTool struct {
	config FileConfig
}

// NewFileTool creates a new FileTool with the given configuration.
func NewFileTool(config FileConfig) *FileTool {
	return &FileTool{config: config}
}

// Name returns "file".
func (f *FileTool) Name() string {
	return "file"
}

// Description returns a description for LLM context.
func (f *FileTool) Description() string {
	return "Reads and writes files within the workspace. " +
		"Payload: {path: string, [content: string]}. " +
		"Use 'file_read' type for reading, 'file_write' for writing."
}

// Validate checks that the action path is within the workspace root.
func (f *FileTool) Validate(action protocol.Action) error {
	path, ok := action.Payload["path"]
	if !ok {
		return fmt.Errorf("file: missing required payload field 'path'")
	}
	pathStr, ok := path.(string)
	if !ok {
		return fmt.Errorf("file: payload 'path' must be a string, got %T", path)
	}
	if strings.TrimSpace(pathStr) == "" {
		return fmt.Errorf("file: path must not be empty")
	}

	// Resolve and verify path is within workspace
	resolved, err := f.resolvePath(pathStr)
	if err != nil {
		return err
	}

	// For write operations, also validate content
	if action.Type == "file_write" {
		if _, ok := action.Payload["content"]; !ok {
			return fmt.Errorf("file_write: missing required payload field 'content'")
		}
	}

	_ = resolved // resolved path is valid
	return nil
}

// Execute performs the file read or write operation.
func (f *FileTool) Execute(action protocol.Action) (protocol.ToolResult, error) {
	start := time.Now()

	pathStr, ok := action.Payload["path"].(string)
	if !ok {
		return protocol.NewErrorResult(action.ID, "invalid path payload", time.Since(start)), nil
	}

	resolvedPath, err := f.resolvePath(pathStr)
	if err != nil {
		return protocol.NewErrorResult(action.ID, err.Error(), time.Since(start)), nil
	}

	switch action.Type {
	case "file_read":
		return f.readFile(action.ID, resolvedPath, start)
	case "file_write":
		content, _ := action.Payload["content"].(string)
		return f.writeFile(action.ID, resolvedPath, content, start)
	default:
		return protocol.NewErrorResult(
			action.ID,
			fmt.Sprintf("unknown file action type: %s", action.Type),
			time.Since(start),
		), nil
	}
}

// resolvePath resolves a path and verifies it is within the workspace root.
func (f *FileTool) resolvePath(rawPath string) (string, error) {
	// Clean the path to remove .. and .
	cleanPath := filepath.Clean(rawPath)

	// Resolve to absolute path
	var absPath string
	if filepath.IsAbs(cleanPath) {
		absPath = cleanPath
	} else {
		absPath = filepath.Join(f.config.WorkspaceRoot, cleanPath)
	}

	// Normalize both paths to absolute form (handles Windows drive letters)
	absRoot, err := filepath.Abs(f.config.WorkspaceRoot)
	if err != nil {
		return "", fmt.Errorf("file: cannot resolve workspace root: %w", err)
	}
	absPath, err = filepath.Abs(absPath)
	if err != nil {
		return "", fmt.Errorf("file: cannot resolve path: %w", err)
	}

	// Ensure the resolved path is within the workspace
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("file: cannot resolve relative path: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("file: path %q is outside workspace root", rawPath)
	}

	return absPath, nil
}

func (f *FileTool) readFile(actionID, path string, start time.Time) (protocol.ToolResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return protocol.NewErrorResult(
			actionID,
			fmt.Sprintf("read failed: %v", err),
			time.Since(start),
		), nil
	}
	return protocol.NewSuccessResult(actionID, string(data), time.Since(start)), nil
}

func (f *FileTool) writeFile(actionID, path, content string, start time.Time) (protocol.ToolResult, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return protocol.NewErrorResult(
			actionID,
			fmt.Sprintf("write failed: %v", err),
			time.Since(start),
		), nil
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return protocol.NewErrorResult(
			actionID,
			fmt.Sprintf("write failed: %v", err),
			time.Since(start),
		), nil
	}

	return protocol.NewSuccessResult(actionID, fmt.Sprintf("wrote %d bytes to %s", len(content), path), time.Since(start)), nil
}
package governance

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// ExecutionBoundary enforces filesystem and workspace boundaries.
type ExecutionBoundary struct {
	workspaceRoot string
}

// NewExecutionBoundary creates a new ExecutionBoundary.
func NewExecutionBoundary(workspaceRoot string) *ExecutionBoundary {
	return &ExecutionBoundary{workspaceRoot: workspaceRoot}
}

// Check verifies that the action does not violate execution boundaries.
func (b *ExecutionBoundary) Check(action protocol.Action) StageResult {
	// Check file paths
	if path, ok := action.Payload["path"].(string); ok {
		if !b.isPathInWorkspace(path) {
			return StageResult{
				StageName: "boundary",
				Passed:    false,
				Reason:    fmt.Sprintf("path %q is outside workspace root %q", path, b.workspaceRoot),
			}
		}
	}

	// Check for shell commands that reference paths outside workspace
	if action.Type == "shell" {
		cmd, _ := action.Payload["command"].(string)
		if b.containsPathOutsideWorkspace(cmd) {
			return StageResult{
				StageName: "boundary",
				Passed:    false,
				Reason:    "shell command references paths outside workspace",
			}
		}
	}

	return StageResult{
		StageName: "boundary",
		Passed:    true,
	}
}

// isPathInWorkspace checks if a path is within the workspace root.
func (b *ExecutionBoundary) isPathInWorkspace(path string) bool {
	if b.workspaceRoot == "" || b.workspaceRoot == "." {
		return true // No workspace restriction
	}

	absRoot, err := filepath.Abs(b.workspaceRoot)
	if err != nil {
		return false
	}

	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(absRoot, path)
	}
	absPath, err = filepath.Abs(absPath)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(rel, "..")
}

// containsPathOutsideWorkspace checks if a shell command references paths outside the workspace.
func (b *ExecutionBoundary) containsPathOutsideWorkspace(cmd string) bool {
	// Check for absolute paths in the command
	parts := strings.Fields(cmd)
	for _, part := range parts {
		if filepath.IsAbs(part) && !b.isPathInWorkspace(part) {
			return true
		}
	}
	return false
}

// ExecutionController manages timeouts and resource limits for tool execution.
type ExecutionController struct {
	toolTimeout time.Duration
}

// NewExecutionController creates a new ExecutionController.
func NewExecutionController(toolTimeout time.Duration) *ExecutionController {
	return &ExecutionController{toolTimeout: toolTimeout}
}

// Check validates that the action can execute within resource constraints.
func (c *ExecutionController) Check(action protocol.Action) StageResult {
	// Check if the action has a timeout that exceeds the global limit
	if timeout, ok := action.Payload["timeout"].(float64); ok {
		if timeout > c.toolTimeout.Seconds() {
			return StageResult{
				StageName: "control",
				Passed:    false,
				Reason:    fmt.Sprintf("action timeout %vs exceeds global limit %v", timeout, c.toolTimeout),
			}
		}
	}

	// Check for resource-intensive operations
	if action.Type == "shell" {
		cmd, _ := action.Payload["command"].(string)
		if isResourceIntensive(cmd) {
			return StageResult{
				StageName:      "control",
				Passed:         false,
				Reason:         "resource-intensive operation detected",
				ShouldEscalate: true,
			}
		}
	}

	return StageResult{
		StageName: "control",
		Passed:    true,
	}
}

// isResourceIntensive checks for commands that may consume excessive resources.
func isResourceIntensive(cmd string) bool {
	intensive := []string{
		"find /", "du /", "tar", "gzip", "npm install", "pip install",
		"docker build", "docker-compose",
	}
	for _, i := range intensive {
		if containsSubstring(cmd, i) {
			return true
		}
	}
	return false
}
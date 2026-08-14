package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// ShellConfig holds configuration for the ShellTool.
type ShellConfig struct {
	// Timeout is the maximum duration for a single command execution.
	// Zero means no timeout.
	Timeout time.Duration
}

// ShellTool executes shell commands on the host system.
// It implements the Tool interface.
type ShellTool struct {
	config ShellConfig
}

// NewShellTool creates a new ShellTool with the given configuration.
func NewShellTool(config ShellConfig) *ShellTool {
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	return &ShellTool{config: config}
}

// Name returns "shell".
func (s *ShellTool) Name() string {
	return "shell"
}

// Description returns a description for LLM context.
func (s *ShellTool) Description() string {
	return "Executes a shell command on the host system. " +
		"Payload: {command: string}. " +
		"Returns stdout on success, stderr on failure."
}

// Validate checks that the action has a valid command payload.
func (s *ShellTool) Validate(action protocol.Action) error {
	cmd, ok := action.Payload["command"]
	if !ok {
		return fmt.Errorf("shell: missing required payload field 'command'")
	}
	cmdStr, ok := cmd.(string)
	if !ok {
		return fmt.Errorf("shell: payload 'command' must be a string, got %T", cmd)
	}
	if strings.TrimSpace(cmdStr) == "" {
		return fmt.Errorf("shell: command must not be empty")
	}
	return nil
}

// Execute runs the shell command and returns the result.
func (s *ShellTool) Execute(action protocol.Action) (protocol.ToolResult, error) {
	start := time.Now()

	cmdStr, ok := action.Payload["command"].(string)
	if !ok {
		return protocol.NewErrorResult(action.ID, "invalid command payload", time.Since(start)), nil
	}

	ctx := context.Background()
	if s.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)

	output, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		return protocol.NewErrorResult(action.ID, "command timed out", elapsed), nil
	}

	if err != nil {
		return protocol.NewErrorResult(
			action.ID,
			fmt.Sprintf("command failed: %v\n%s", err, string(output)),
			elapsed,
		), nil
	}

	return protocol.NewSuccessResult(action.ID, string(output), elapsed), nil
}
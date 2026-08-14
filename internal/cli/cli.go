package cli

import (
	"context"
	"fmt"

	"github.com/tyXiang-520/CageHarness/internal/runtime"
)

// CLI is a thin client around the runtime layer.
//
// Architecture:
//
//	CLI (this package)
//	  │
//	  ↓
//	TaskManager / AgentLoop (runtime package)
//	  │
//	  ↓
//	Governance / Tools / LLM (domain packages)
//
// CLI does NOT import agent, feedback, memory, governance, tools, llm,
// or protocol directly. It is a runtime client, not a second runtime.
type CLI struct {
	taskManager *runtime.TaskManager
	loop        *runtime.AgentLoop
}

// NewCLI creates a new CLI with the given runtime dependencies.
func NewCLI(tm *runtime.TaskManager, loop *runtime.AgentLoop) *CLI {
	return &CLI{
		taskManager: tm,
		loop:        loop,
	}
}

// Run executes a task synchronously: submits it and waits for the result.
// This is the simple "fire and wait" mode for CLI usage.
func (c *CLI) Run(ctx context.Context, task string) (string, error) {
	resultCh := make(chan struct {
		result string
		err    error
	}, 1)

	c.taskManager.Submit(ctx, task, func(ctx context.Context) (string, error) {
		result, err := c.loop.Run(ctx, task)
		resultCh <- struct {
			result string
			err    error
		}{result, err}
		return result, err
	})

	r := <-resultCh
	return r.result, r.err
}

// Submit creates a task asynchronously and returns the task ID.
// Use Status() to check progress.
func (c *CLI) Submit(ctx context.Context, task string) string {
	return c.taskManager.Submit(ctx, task, func(ctx context.Context) (string, error) {
		return c.loop.Run(ctx, task)
	})
}

// Status returns the current status of a task.
func (c *CLI) Status(taskID string) (*runtime.Task, error) {
	task, ok := c.taskManager.Get(taskID)
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return task, nil
}

// List returns all tasks.
func (c *CLI) List() []*runtime.Task {
	return c.taskManager.List()
}

// Cancel cancels a running task.
func (c *CLI) Cancel(taskID string) error {
	return c.taskManager.Cancel(taskID)
}
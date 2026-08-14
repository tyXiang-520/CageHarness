package cli

import (
	"context"
	"testing"

	"github.com/tyXiang-520/CageHarness/internal/governance"
	"github.com/tyXiang-520/CageHarness/internal/llm"
	"github.com/tyXiang-520/CageHarness/internal/runtime"
	"github.com/tyXiang-520/CageHarness/internal/tools"
)

// setupTestCLI creates a CLI with mock dependencies for testing.
func setupTestCLI(t *testing.T) *CLI {
	t.Helper()

	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "hello from cli test"),
			llm.FinishReasonStop,
		), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()
	tm := runtime.NewTaskManager()

	loop := runtime.NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, runtime.LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "test",
	})

	return NewCLI(tm, loop)
}

func TestCLI_Run(t *testing.T) {
	// Run is synchronous: submit a task and wait for the result
	cli := setupTestCLI(t)

	ctx := context.Background()
	result, err := cli.Run(ctx, "say hello")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result != "hello from cli test" {
		t.Errorf("expected 'hello from cli test', got %s", result)
	}
}

func TestCLI_SubmitAndStatus(t *testing.T) {
	// Submit is async: returns task ID, then query status
	cli := setupTestCLI(t)

	ctx := context.Background()
	taskID := cli.Submit(ctx, "async task")

	// Task should exist immediately
	task, err := cli.Status(taskID)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if task.ID != taskID {
		t.Errorf("expected task ID %s, got %s", taskID, task.ID)
	}
	if task.Task != "async task" {
		t.Errorf("expected task 'async task', got %s", task.Task)
	}

	// Wait for completion
	cli.taskManager.Wait()

	task, err = cli.Status(taskID)
	if err != nil {
		t.Fatalf("Status after completion failed: %v", err)
	}
	if task.Status != runtime.TaskStatusCompleted {
		t.Errorf("expected Completed, got %s", task.Status)
	}
}

func TestCLI_StatusNotFound(t *testing.T) {
	cli := setupTestCLI(t)

	_, err := cli.Status("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestCLI_List(t *testing.T) {
	cli := setupTestCLI(t)
	ctx := context.Background()

	cli.Submit(ctx, "task one")
	cli.Submit(ctx, "task two")

	cli.taskManager.Wait()

	tasks := cli.List()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestCLI_Cancel(t *testing.T) {
	cli := setupTestCLI(t)

	// Set up a mock that blocks until cancelled
	cli.loop = runtime.NewAgentLoop(
		newBlockingMock(),
		governance.NewPipeline(governance.DefaultGovernanceContext()),
		tools.NewRegistry(),
		runtime.LoopConfig{
			MaxIterations: 5,
			SystemPrompt:  "test",
		},
	)

	ctx := context.Background()
	taskID := cli.Submit(ctx, "cancellable task")

	// Cancel the task
	if err := cli.Cancel(taskID); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	cli.taskManager.Wait()

	task, err := cli.Status(taskID)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if task.Status != runtime.TaskStatusCancelled {
		t.Errorf("expected Cancelled, got %s", task.Status)
	}
}

func TestCLI_CancelNotFound(t *testing.T) {
	cli := setupTestCLI(t)
	err := cli.Cancel("nonexistent")
	if err == nil {
		t.Error("expected error for cancelling nonexistent task")
	}
}

func TestCLI_IsRuntimeClient(t *testing.T) {
	// ★★★ KEY TEST ★★★
	// Verifies that CLI is a thin client, not a second runtime.
	// CLI does NOT import agent, feedback, memory, or protocol directly.
	// CLI only talks to runtime.TaskManager and runtime.AgentLoop.
	cli := setupTestCLI(t)

	// CLI.Run() delegates to TaskManager which delegates to AgentLoop
	// CLI does not call llm.Generate, governance.Evaluate, or tools.Execute directly
	ctx := context.Background()
	result, err := cli.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}

	// Verify the task lifecycle went through TaskManager
	tasks := cli.List()
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Status != runtime.TaskStatusCompleted {
		t.Errorf("expected Completed, got %s", tasks[0].Status)
	}
}

// blockingMock is an LLM provider that blocks until context is cancelled.
type blockingMock struct{}

func (b *blockingMock) Generate(ctx context.Context, messages []llm.Message) (llm.Response, error) {
	<-ctx.Done()
	return llm.Response{}, ctx.Err()
}

func newBlockingMock() *blockingMock {
	return &blockingMock{}
}

// Ensure blockingMock implements llm.Provider
var _ llm.Provider = (*blockingMock)(nil)
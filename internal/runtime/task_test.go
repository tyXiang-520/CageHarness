package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/governance"
	"github.com/tyXiang-520/CageHarness/internal/llm"
	"github.com/tyXiang-520/CageHarness/internal/tools"
)

func TestTaskManager_SubmitAndComplete(t *testing.T) {
	// Basic lifecycle: Submit → Running → Completed
	tm := NewTaskManager()

	ctx := context.Background()
	taskID := tm.Submit(ctx, "test task", func(ctx context.Context) (string, error) {
		return "task result", nil
	})

	// Wait for task to complete
	tm.wg.Wait()

	task, ok := tm.Get(taskID)
	if !ok {
		t.Fatal("task should exist after submission")
	}

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected Completed, got %s", task.Status)
	}
	if task.Result != "task result" {
		t.Errorf("expected 'task result', got %s", task.Result)
	}
	if task.Error != "" {
		t.Errorf("expected no error, got %s", task.Error)
	}
	if task.Task != "test task" {
		t.Errorf("expected 'test task', got %s", task.Task)
	}
}

func TestTaskManager_LifecycleTransitions(t *testing.T) {
	// Verify the full lifecycle: Pending → Running → Completed
	tm := NewTaskManager()

	started := make(chan struct{})
	ctx := context.Background()

	taskID := tm.Submit(ctx, "lifecycle test", func(ctx context.Context) (string, error) {
		started <- struct{}{}
		return "done", nil
	})

	// Immediately after Submit, task should be Pending or Running
	task, ok := tm.Get(taskID)
	if !ok {
		t.Fatal("task should exist")
	}
	if task.Status != TaskStatusPending && task.Status != TaskStatusRunning {
		t.Errorf("expected Pending or Running, got %s", task.Status)
	}

	// Wait for task to start
	<-started
	tm.wg.Wait()

	// After completion, task should be Completed
	task, ok = tm.Get(taskID)
	if !ok {
		t.Fatal("task should still exist")
	}
	if task.Status != TaskStatusCompleted {
		t.Errorf("expected Completed, got %s", task.Status)
	}
}

func TestTaskManager_TaskFailed(t *testing.T) {
	// Task function returns an error → status should be Failed
	tm := NewTaskManager()

	ctx := context.Background()
	taskID := tm.Submit(ctx, "failing task", func(ctx context.Context) (string, error) {
		return "", errors.New("something went wrong")
	})

	tm.wg.Wait()

	task, ok := tm.Get(taskID)
	if !ok {
		t.Fatal("task should exist")
	}
	if task.Status != TaskStatusFailed {
		t.Errorf("expected Failed, got %s", task.Status)
	}
	if task.Error != "something went wrong" {
		t.Errorf("expected error message, got %s", task.Error)
	}
}

func TestTaskManager_Cancel(t *testing.T) {
	// Cancel a running task → status should be Cancelled
	tm := NewTaskManager()

	started := make(chan struct{})
	ctx := context.Background()

	taskID := tm.Submit(ctx, "cancellable task", func(ctx context.Context) (string, error) {
		started <- struct{}{}
		// Simulate long-running work that respects context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Second):
			return "finished", nil
		}
	})

	// Wait for the task to start
	<-started

	// Cancel the task
	if err := tm.Cancel(taskID); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	tm.wg.Wait()

	task, ok := tm.Get(taskID)
	if !ok {
		t.Fatal("task should exist after cancellation")
	}
	if task.Status != TaskStatusCancelled {
		t.Errorf("expected Cancelled, got %s", task.Status)
	}
}

func TestTaskManager_CancelNonexistent(t *testing.T) {
	tm := NewTaskManager()
	err := tm.Cancel("nonexistent-id")
	if err == nil {
		t.Error("expected error when cancelling nonexistent task")
	}
}

func TestTaskManager_GetNonexistent(t *testing.T) {
	tm := NewTaskManager()
	_, ok := tm.Get("nonexistent-id")
	if ok {
		t.Error("Get should return false for nonexistent task")
	}
}

func TestTaskManager_List(t *testing.T) {
	tm := NewTaskManager()
	ctx := context.Background()

	// Submit multiple tasks
	tm.Submit(ctx, "task-a", func(ctx context.Context) (string, error) {
		return "a", nil
	})
	tm.Submit(ctx, "task-b", func(ctx context.Context) (string, error) {
		return "b", nil
	})

	tm.wg.Wait()

	tasks := tm.List()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestTaskManager_ListByOwner(t *testing.T) {
	tm := NewTaskManager()
	ctx := context.Background()

	// Default Submit uses the empty owner
	tm.Submit(ctx, "no-owner", func(ctx context.Context) (string, error) {
		return "", nil
	})
	tm.SubmitWithOwner(ctx, "alice", "alice-task", func(ctx context.Context) (string, error) {
		return "", nil
	})
	tm.SubmitWithOwner(ctx, "bob", "bob-task", func(ctx context.Context) (string, error) {
		return "", nil
	})

	tm.wg.Wait()

	// Full list contains everything
	if got := len(tm.List()); got != 3 {
		t.Errorf("List should return 3 tasks, got %d", got)
	}

	// Per-owner views are isolated
	alice := tm.ListByOwner("alice")
	if len(alice) != 1 || alice[0].Task != "alice-task" {
		t.Errorf("alice should see only her task, got %v", alice)
	}
	bob := tm.ListByOwner("bob")
	if len(bob) != 1 || bob[0].Task != "bob-task" {
		t.Errorf("bob should see only his task, got %v", bob)
	}
	empty := tm.ListByOwner("")
	if len(empty) != 1 || empty[0].Task != "no-owner" {
		t.Errorf("empty owner should see only the ownerless task, got %v", empty)
	}

	// Owner is stored on the task itself
	if alice[0].Owner != "alice" {
		t.Errorf("expected owner alice, got %q", alice[0].Owner)
	}
}

func TestTaskManager_Concurrency(t *testing.T) {
	// Verify concurrent task creation is safe
	tm := NewTaskManager()
	ctx := context.Background()

	var wg sync.WaitGroup
	numTasks := 20

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tm.Submit(ctx, "concurrent task", func(ctx context.Context) (string, error) {
				return "ok", nil
			})
		}(i)
	}

	wg.Wait()     // Wait for all submissions
	tm.wg.Wait()  // Wait for all tasks to complete

	tasks := tm.List()
	if len(tasks) != numTasks {
		t.Errorf("expected %d tasks, got %d", numTasks, len(tasks))
	}

	// Verify all tasks completed
	for _, task := range tasks {
		if task.Status != TaskStatusCompleted {
			t.Errorf("task %s: expected Completed, got %s", task.ID, task.Status)
		}
	}
}

func TestTaskManager_AgentStateTracking(t *testing.T) {
	// Verify that TaskManager correctly manages an AgentLoop execution
	tm := NewTaskManager()
	ctx := context.Background()

	// Create a minimal AgentLoop inline
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "hello from mock"),
			llm.FinishReasonStop,
		), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "test",
	})

	taskID := tm.Submit(ctx, "agent task", func(ctx context.Context) (string, error) {
		return loop.Run(ctx, "say hello")
	})

	tm.wg.Wait()

	task, ok := tm.Get(taskID)
	if !ok {
		t.Fatal("task should exist")
	}
	if task.Status != TaskStatusCompleted {
		t.Errorf("expected Completed, got %s", task.Status)
	}
	if task.Result != "hello from mock" {
		t.Errorf("expected 'hello from mock', got %s", task.Result)
	}
}

func TestTaskManager_TaskIDsAreUnique(t *testing.T) {
	tm := NewTaskManager()
	ctx := context.Background()

	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		id := tm.Submit(ctx, "unique test", func(ctx context.Context) (string, error) {
			return "ok", nil
		})
		if ids[id] {
			t.Errorf("duplicate task ID: %s", id)
		}
		ids[id] = true
	}

	tm.wg.Wait()
}

func TestTaskManager_CreatedAtAndUpdatedAt(t *testing.T) {
	tm := NewTaskManager()
	ctx := context.Background()

	before := time.Now()
	taskID := tm.Submit(ctx, "timestamp test", func(ctx context.Context) (string, error) {
		return "ok", nil
	})
	tm.wg.Wait()

	task, ok := tm.Get(taskID)
	if !ok {
		t.Fatal("task should exist")
	}

	if task.CreatedAt.Before(before) {
		t.Error("CreatedAt should be after submission time")
	}
	if task.UpdatedAt.Before(task.CreatedAt) {
		t.Error("UpdatedAt should be after or equal to CreatedAt")
	}
}

func TestTaskManager_StatusString(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected string
	}{
		{TaskStatusPending, "pending"},
		{TaskStatusRunning, "running"},
		{TaskStatusCompleted, "completed"},
		{TaskStatusFailed, "failed"},
		{TaskStatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.expected {
			t.Errorf("TaskStatus(%d).String() = %s, want %s", tt.status, tt.expected, got)
		}
	}
}
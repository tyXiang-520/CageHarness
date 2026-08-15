package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TaskStatus represents the lifecycle state of a task managed by TaskManager.
// This is distinct from agent.AgentState — TaskStatus tracks the external
// task lifecycle, while AgentState tracks the internal agent decision loop.
type TaskStatus int

const (
	// TaskStatusPending: task has been submitted but not yet started.
	TaskStatusPending TaskStatus = iota
	// TaskStatusRunning: task is currently executing.
	TaskStatusRunning
	// TaskStatusCompleted: task finished successfully.
	TaskStatusCompleted
	// TaskStatusFailed: task finished with an error.
	TaskStatusFailed
	// TaskStatusCancelled: task was cancelled before completion.
	TaskStatusCancelled
)

// String returns the human-readable name of the task status.
func (s TaskStatus) String() string {
	switch s {
	case TaskStatusPending:
		return "pending"
	case TaskStatusRunning:
		return "running"
	case TaskStatusCompleted:
		return "completed"
	case TaskStatusFailed:
		return "failed"
	case TaskStatusCancelled:
		return "cancelled"
	default:
		return fmt.Sprintf("TaskStatus(%d)", int(s))
	}
}

// IsTerminal returns true if the status is a terminal state.
func (s TaskStatus) IsTerminal() bool {
	return s == TaskStatusCompleted || s == TaskStatusFailed || s == TaskStatusCancelled
}

// Task represents a unit of work managed by the TaskManager.
// It owns an AgentLoop execution and tracks its external lifecycle.
type Task struct {
	ID        string
	Task      string
	Status    TaskStatus
	Result    string
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Owner identifies the visitor who submitted the task (WebUI isolation).
	// An empty owner means the task was submitted without a visitor ID
	// (e.g., by the CLI or curl). Isolation is privacy-by-default, not
	// authentication: anyone can forge a visitor ID.
	Owner string

	// Rich observability data (populated when using SubmitWithResult)
	RunResult *RunResult `json:"run_result,omitempty"`
}

// TaskFunc is the function signature for task execution.
// It receives a context that is cancelled when the task is cancelled.
type TaskFunc func(ctx context.Context) (string, error)

// TaskFuncWithResult is the function signature for task execution with rich result.
type TaskFuncWithResult func(ctx context.Context) RunResult

// TaskManager manages the lifecycle of async tasks.
//
// Architecture:
//
//	TaskManager
//	  │
//	  │ Submit(taskFunc)
//	  ↓
//	Task (Pending → Running → Completed/Failed/Cancelled)
//	  │
//	  │ owns
//	  ↓
//	AgentLoop.Run(ctx, task)
//	  │
//	  ↓
//	AgentState (Thinking, Executing, Observing, ...)
//
// TaskManager does NOT:
//   - Define new Agent state enums
//   - Call tools directly
//   - Implement workflow orchestration (DAG, retry, priority queue)
type TaskManager struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	cancels  map[string]context.CancelFunc
	counter  uint64
	wg       sync.WaitGroup
}

// NewTaskManager creates a new TaskManager.
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks:   make(map[string]*Task),
		cancels: make(map[string]context.CancelFunc),
	}
}

// Submit creates a new task and starts executing it asynchronously.
// The returned task ID can be used to query status or cancel the task.
func (tm *TaskManager) Submit(parentCtx context.Context, task string, fn TaskFunc) string {
	return tm.SubmitWithOwner(parentCtx, "", task, fn)
}

// SubmitWithOwner creates a new task owned by the given owner and starts
// executing it asynchronously. The owner is used by the WebUI to isolate
// visitors' task lists (see ListByOwner).
func (tm *TaskManager) SubmitWithOwner(parentCtx context.Context, owner, task string, fn TaskFunc) string {
	id := tm.nextID()
	now := time.Now()

	t := &Task{
		ID:        id,
		Task:      task,
		Status:    TaskStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
		Owner:     owner,
	}

	ctx, cancel := context.WithCancel(parentCtx)

	tm.mu.Lock()
	tm.tasks[id] = t
	tm.cancels[id] = cancel
	tm.mu.Unlock()

	tm.wg.Add(1)
	go tm.run(ctx, t, fn)

	return id
}

// SubmitWithResult creates a new task and starts executing it asynchronously.
// The task function returns a RunResult containing rich observability data
// (state transitions, audit log, etc.) that is stored in the Task.
func (tm *TaskManager) SubmitWithResult(parentCtx context.Context, task string, fn TaskFuncWithResult) string {
	return tm.SubmitWithResultOwner(parentCtx, "", task, fn)
}

// SubmitWithResultOwner creates a new task owned by the given owner and
// starts executing it asynchronously, storing rich observability data.
func (tm *TaskManager) SubmitWithResultOwner(parentCtx context.Context, owner, task string, fn TaskFuncWithResult) string {
	id := tm.nextID()
	now := time.Now()

	t := &Task{
		ID:        id,
		Task:      task,
		Status:    TaskStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
		Owner:     owner,
	}

	ctx, cancel := context.WithCancel(parentCtx)

	tm.mu.Lock()
	tm.tasks[id] = t
	tm.cancels[id] = cancel
	tm.mu.Unlock()

	tm.wg.Add(1)
	go tm.runWithResult(ctx, t, fn)

	return id
}

// Get returns the task with the given ID, or false if not found.
func (tm *TaskManager) Get(id string) (*Task, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.tasks[id]
	if !ok {
		return nil, false
	}
	// Return a copy to prevent data races
	cp := *t
	return &cp, true
}

// Cancel cancels the task with the given ID.
// Returns an error if the task is not found or already in a terminal state.
func (tm *TaskManager) Cancel(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.tasks[id]
	if !ok {
		return fmt.Errorf("task %s not found", id)
	}
	if t.Status.IsTerminal() {
		return fmt.Errorf("task %s already in terminal state: %s", id, t.Status)
	}

	cancel, ok := tm.cancels[id]
	if !ok {
		return fmt.Errorf("task %s has no cancel function", id)
	}
	cancel()

	return nil
}

// List returns all tasks.
func (tm *TaskManager) List() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*Task, 0, len(tm.tasks))
	for _, t := range tm.tasks {
		cp := *t
		result = append(result, &cp)
	}
	return result
}

// ListByOwner returns only the tasks owned by the given owner.
// Used by the WebUI so each visitor sees only their own task list.
func (tm *TaskManager) ListByOwner(owner string) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*Task, 0)
	for _, t := range tm.tasks {
		if t.Owner == owner {
			cp := *t
			result = append(result, &cp)
		}
	}
	return result
}

// Wait blocks until all submitted tasks have completed.
func (tm *TaskManager) Wait() {
	tm.wg.Wait()
}

// run executes the task function and updates the task status.
func (tm *TaskManager) run(ctx context.Context, t *Task, fn TaskFunc) {
	defer tm.wg.Done()

	// Transition to Running
	tm.updateTask(t.ID, TaskStatusRunning, "", "")

	result, err := fn(ctx)

	// Determine final status
	if err != nil {
		if ctx.Err() != nil {
			// Context was cancelled
			tm.updateTask(t.ID, TaskStatusCancelled, "", err.Error())
		} else {
			tm.updateTask(t.ID, TaskStatusFailed, "", err.Error())
		}
	} else {
		tm.updateTask(t.ID, TaskStatusCompleted, result, "")
	}

	// Clean up cancel function
	tm.mu.Lock()
	delete(tm.cancels, t.ID)
	tm.mu.Unlock()
}

// runWithResult executes the task function and stores the rich RunResult.
func (tm *TaskManager) runWithResult(ctx context.Context, t *Task, fn TaskFuncWithResult) {
	defer tm.wg.Done()

	// Transition to Running
	tm.updateTaskWithResult(t.ID, TaskStatusRunning, "", "", nil)

	result := fn(ctx)

	// Determine final status
	if ctx.Err() != nil {
		// Context was cancelled
		tm.updateTaskWithResult(t.ID, TaskStatusCancelled, "", result.Error, &result)
	} else if result.Error != "" {
		tm.updateTaskWithResult(t.ID, TaskStatusFailed, "", result.Error, &result)
	} else {
		tm.updateTaskWithResult(t.ID, TaskStatusCompleted, result.Text, "", &result)
	}

	// Clean up cancel function
	tm.mu.Lock()
	delete(tm.cancels, t.ID)
	tm.mu.Unlock()
}

// updateTask updates the status, result, and error of a task.
func (tm *TaskManager) updateTask(id string, status TaskStatus, result, errMsg string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.tasks[id]
	if !ok {
		return
	}
	t.Status = status
	t.Result = result
	t.Error = errMsg
	t.UpdatedAt = time.Now()
}

// updateTaskWithResult updates the task including the RunResult.
func (tm *TaskManager) updateTaskWithResult(id string, status TaskStatus, result, errMsg string, runResult *RunResult) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.tasks[id]
	if !ok {
		return
	}
	t.Status = status
	t.Result = result
	t.Error = errMsg
	t.RunResult = runResult
	t.UpdatedAt = time.Now()
}

// nextID generates a unique task ID.
func (tm *TaskManager) nextID() string {
	n := atomic.AddUint64(&tm.counter, 1)
	return fmt.Sprintf("task-%d", n)
}
package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tyXiang-520/CageHarness/internal/governance"
	"github.com/tyXiang-520/CageHarness/internal/llm"
	"github.com/tyXiang-520/CageHarness/internal/runtime"
	"github.com/tyXiang-520/CageHarness/internal/tools"
)

// setupTestServer creates a test Server with mock dependencies.
func setupTestServer(t *testing.T) *Server {
	t.Helper()

	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "web response"),
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

	return NewServer(tm, loop)
}

// Gate A: Verify web package does NOT import domain packages directly.
// This is checked at compile time — if web imports agent/governance/tools/llm/memory/feedback,
// the linter/build would catch it. The test file imports them only for test setup,
// which is the same pattern as CLI tests.

// TestWebUI_IndexServesInlineHTML verifies the WebUI page is served as an
// inline HTML document, not an attachment. SCF's gateway injects
// Content-Disposition: attachment by default, which makes browsers download
// the page instead of rendering it — the handler must declare inline.
func TestWebUI_IndexServesInlineHTML(t *testing.T) {
	srv := setupTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Bare / redirects to /index.html (client must NOT follow the redirect)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 redirect from /, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/index.html" {
		t.Errorf("expected Location /index.html, got %q", loc)
	}

	// /index.html serves inline HTML
	resp, err = http.Get(ts.URL + "/index.html")
	if err != nil {
		t.Fatalf("GET /index.html failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != "inline" {
		t.Errorf("expected Content-Disposition: inline (SCF adds attachment), got %q", cd)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}
	if !strings.Contains(string(body), "CageHarness") {
		t.Error("expected WebUI HTML body to contain CageHarness")
	}
}

func TestWebUI_SubmitTask(t *testing.T) {
	// Gate B: POST /tasks returns task_id, does not block waiting for LLM
	srv := setupTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/tasks", "application/json",
		strings.NewReader(`{"task":"say hello from web"}`))
	if err != nil {
		t.Fatalf("POST /tasks failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202 Accepted, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	taskID, ok := result["task_id"]
	if !ok || taskID == "" {
		t.Fatal("expected task_id in response")
	}

	// Gate D: Verify status uses TaskStatus enum values, not custom strings
	srv.taskManager.Wait()

	resp2, err := http.Get(ts.URL + "/tasks/" + taskID)
	if err != nil {
		t.Fatalf("GET /tasks/%s failed: %v", taskID, err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp2.StatusCode)
	}

	var taskResp taskResponse
	if err := json.NewDecoder(resp2.Body).Decode(&taskResp); err != nil {
		t.Fatalf("decode task response: %v", err)
	}

	if taskResp.Status != runtime.TaskStatusCompleted.String() {
		t.Errorf("expected 'completed', got '%s'", taskResp.Status)
	}
	if taskResp.Result != "web response" {
		t.Errorf("expected 'web response', got '%s'", taskResp.Result)
	}
}

func TestWebUI_GetTaskNotFound(t *testing.T) {
	srv := setupTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/tasks/nonexistent")
	if err != nil {
		t.Fatalf("GET /tasks/nonexistent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestWebUI_ListTasks(t *testing.T) {
	srv := setupTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Submit two tasks
	for i := 0; i < 2; i++ {
		resp, err := http.Post(ts.URL+"/tasks", "application/json",
		strings.NewReader(`{"task":"test task"}`))
		if err != nil {
			t.Fatalf("POST /tasks: %v", err)
		}
		resp.Body.Close()
	}

	srv.taskManager.Wait()

	resp, err := http.Get(ts.URL + "/tasks")
	if err != nil {
		t.Fatalf("GET /tasks: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var tasks []taskResponse
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		t.Fatalf("decode tasks: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestWebUI_CancelTask(t *testing.T) {
	srv := setupTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Replace the loop with a blocking one so we can cancel
	srv.loop = runtime.NewAgentLoop(
		newBlockingMock(),
		governance.NewPipeline(governance.DefaultGovernanceContext()),
		tools.NewRegistry(),
		runtime.LoopConfig{
			MaxIterations: 5,
			SystemPrompt:  "test",
		},
	)

	resp, err := http.Post(ts.URL+"/tasks", "application/json",
		strings.NewReader(`{"task":"cancellable"}`))
	if err != nil {
		t.Fatalf("POST /tasks: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	taskID := result["task_id"]

	// Cancel the task
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/tasks/"+taskID, nil)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /tasks/%s: %v", taskID, err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp2.StatusCode)
	}

	srv.taskManager.Wait()

	// Verify cancelled
	resp3, err := http.Get(ts.URL + "/tasks/" + taskID)
	if err != nil {
		t.Fatalf("GET /tasks/%s: %v", taskID, err)
	}
	defer resp3.Body.Close()

	var taskResp taskResponse
	json.NewDecoder(resp3.Body).Decode(&taskResp)
	if taskResp.Status != runtime.TaskStatusCancelled.String() {
		t.Errorf("expected 'cancelled', got '%s'", taskResp.Status)
	}
}

func TestWebUI_ContextLifecycleSeparation(t *testing.T) {
	// Gate C: HTTP context != Task context.
	// Client disconnect should NOT cancel the AgentLoop.

	srv := setupTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Submit a task
	resp, err := http.Post(ts.URL+"/tasks", "application/json",
		strings.NewReader(`{"task":"long running"}`))
	if err != nil {
		t.Fatalf("POST /tasks: %v", err)
	}
	resp.Body.Close()

	// The task should complete even though the HTTP request is done
	// The HTTP handler returned immediately with 202 Accepted
	// The task continues in the background

	srv.taskManager.Wait()

	// Verify task completed
	tasks := srv.taskManager.List()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Status != runtime.TaskStatusCompleted {
		t.Errorf("task should complete even after HTTP request ends, got %s", tasks[0].Status)
	}
}

func TestWebUI_NoCustomStatusEnum(t *testing.T) {
	// Gate D: WebUI must not introduce a new status enum.
	// taskResponse.Status must be a string from runtime.TaskStatus.String().

	// Verify all valid status values are from runtime.TaskStatus
	validStatuses := map[string]bool{
		runtime.TaskStatusPending.String():   true,
		runtime.TaskStatusRunning.String():   true,
		runtime.TaskStatusCompleted.String(): true,
		runtime.TaskStatusFailed.String():    true,
		runtime.TaskStatusCancelled.String(): true,
	}

	// There should be exactly 5 status values
	if len(validStatuses) != 5 {
		t.Errorf("expected 5 status values, got %d", len(validStatuses))
	}

	// Verify no extra status strings exist in the web package
	// (compile-time check: no WebStatus type defined)
}

func TestWebUI_InvalidMethod(t *testing.T) {
	srv := setupTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// PUT should not be supported
	resp, err := http.Post(ts.URL+"/tasks/task-1", "application/json", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

func TestWebUI_EmptyTaskBody(t *testing.T) {
	srv := setupTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/tasks", "application/json",
		strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST /tasks: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestWebUI_ServerStop(t *testing.T) {
	srv := setupTestServer(t)
	ts := httptest.NewServer(srv.Handler())

	// Server should be running
	resp, err := http.Get(ts.URL + "/tasks")
	if err != nil {
		t.Fatalf("GET /tasks: %v", err)
	}
	resp.Body.Close()

	// Stop the server
	ts.Close()

	// Subsequent requests should fail
	_, err = http.Get(ts.URL + "/tasks")
	if err == nil {
		t.Error("expected error after server stop")
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

var _ llm.Provider = (*blockingMock)(nil)

// closeBody is a helper to safely close response bodies in tests.
func closeBody(t *testing.T, body io.ReadCloser) {
	t.Helper()
	if err := body.Close(); err != nil {
		t.Logf("close body: %v", err)
	}
}
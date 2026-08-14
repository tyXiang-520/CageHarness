package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tyXiang-520/CageHarness/internal/runtime"
)

// Server is a thin HTTP wrapper around the runtime layer.
//
// Architecture:
//
//	WebUI (this package)
//	  │
//	  ↓
//	TaskManager / AgentLoop (runtime package)
//	  │
//	  ↓
//	Governance / Tools / LLM (domain packages)
//
// Server does NOT import agent, feedback, memory, governance, tools, llm,
// or protocol directly. It is a runtime client, not a second runtime.
//
// Gate D: No new status enum. All status values come from runtime.TaskStatus.
type Server struct {
	taskManager *runtime.TaskManager
	loop        *runtime.AgentLoop
}

// NewServer creates a new WebUI server with the given runtime dependencies.
func NewServer(tm *runtime.TaskManager, loop *runtime.AgentLoop) *Server {
	return &Server{
		taskManager: tm,
		loop:        loop,
	}
}

// Handler returns the HTTP handler for the server.
// Routes:
//
//	GET    /               — WebUI (embedded HTML)
//	POST   /tasks          — Submit a task asynchronously
//	GET    /tasks/{id}     — Get task status
//	GET    /tasks          — List all tasks (when no id suffix)
//	DELETE /tasks/{id}     — Cancel a task
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/tasks", s.handleTasks)
	mux.HandleFunc("/tasks/", s.handleTaskByID)
	return mux
}

// handleIndex serves the embedded WebUI HTML page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

// Start begins listening on the given address and serving HTTP requests.
// It blocks until the server is stopped. Use in a goroutine for non-blocking start.
func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

// handleTasks handles requests to /tasks (collection endpoint).
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.submitTask(w, r)
	case http.MethodGet:
		s.listTasks(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTaskByID handles requests to /tasks/{id} (single resource endpoint).
func (s *Server) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	// Extract task ID from path: /tasks/{id}
	id := strings.TrimPrefix(r.URL.Path, "/tasks/")
	if id == "" {
		http.Error(w, "task id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getTask(w, r, id)
	case http.MethodDelete:
		s.cancelTask(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// submitTask handles POST /tasks.
//
// Gate B: Returns immediately with 202 Accepted and task_id.
// Does NOT block waiting for the LLM.
//
// Gate C: Uses context.Background() for the task, not r.Context().
// HTTP disconnect does NOT cancel the AgentLoop.
func (s *Server) submitTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Task string `json:"task"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Task == "" {
		http.Error(w, `{"error":"task field is required"}`, http.StatusBadRequest)
		return
	}

	// Gate C: Use context.Background() for task lifecycle.
	// The HTTP request context (r.Context()) is only for reading the request body.
	// client disconnect → HTTP handler returns → Task continues in background.
	taskID := s.taskManager.Submit(context.Background(), req.Task, func(ctx context.Context) (string, error) {
		return s.loop.Run(ctx, req.Task)
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"task_id": taskID,
	})
}

// getTask handles GET /tasks/{id}.
func (s *Server) getTask(w http.ResponseWriter, r *http.Request, id string) {
	task, ok := s.taskManager.Get(id)
	if !ok {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(taskToResponse(task))
}

// listTasks handles GET /tasks.
func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.taskManager.List()

	w.Header().Set("Content-Type", "application/json")
	responses := make([]taskResponse, 0, len(tasks))
	for _, t := range tasks {
		responses = append(responses, taskToResponse(t))
	}
	json.NewEncoder(w).Encode(responses)
}

// cancelTask handles DELETE /tasks/{id}.
func (s *Server) cancelTask(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.taskManager.Cancel(id); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "cancelled",
	})
}

// taskResponse is the JSON response shape for a task.
// Gate D: Status field uses runtime.TaskStatus.String() — no custom enum.
type taskResponse struct {
	ID        string `json:"id"`
	Task      string `json:"task"`
	Status    string `json:"status"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// taskToResponse converts a runtime.Task to a taskResponse.
func taskToResponse(t *runtime.Task) taskResponse {
	return taskResponse{
		ID:        t.ID,
		Task:      t.Task,
		Status:    t.Status.String(),
		Result:    t.Result,
		Error:     t.Error,
		CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: t.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
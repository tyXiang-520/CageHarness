package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/agent"
	"github.com/tyXiang-520/CageHarness/internal/governance"
	"github.com/tyXiang-520/CageHarness/internal/llm"
	"github.com/tyXiang-520/CageHarness/internal/protocol"
	"github.com/tyXiang-520/CageHarness/internal/tools"
)

// mockTool is a simple tool that returns a fixed result.
// Used to test the Agent Loop without real filesystem or shell access.
type mockTool struct {
	name        string
	description string
	result      string
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return m.description }
func (m *mockTool) Execute(action protocol.Action) (protocol.ToolResult, error) {
	return protocol.NewSuccessResult(action.ID, m.result, 0), nil
}
func (m *mockTool) Validate(action protocol.Action) error { return nil }

func TestAgentLoop_SimpleCompletion(t *testing.T) {
	// LLM returns a text response (no tool calls) → loop should terminate
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "Hello, I can help you with that!"),
			llm.FinishReasonStop,
		), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "You are a helpful assistant.",
	})

	ctx := context.Background()
	result, err := loop.Run(ctx, "Say hello")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result != "Hello, I can help you with that!" {
		t.Errorf("unexpected result: %s", result)
	}

	// Verify state transitions occurred
	if loop.state != agent.AgentStateTerminated {
		t.Errorf("expected terminated state, got %s", loop.state)
	}
}

func TestAgentLoop_SingleToolCall(t *testing.T) {
	// LLM returns a tool call → governance allows → tool executes → loop continues → LLM returns text
	callCount := 0
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		callCount++
		if callCount == 1 {
			// First call: return a tool call
			msg := llm.Message{Role: llm.RoleAssistant, Content: "Let me check something"}
			msg.WithToolCall("call-1", "mock", `{"input":"test"}`)
			return llm.NewToolCallResponse("Let me check something", msg.ToolCalls...), nil
		}
		// Second call: final response
		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "The result is: mock result"),
			llm.FinishReasonStop,
		), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()
	toolReg.Register(&mockTool{name: "mock", description: "mock tool", result: "mock result"})

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "You are a helpful assistant.",
	})

	ctx := context.Background()
	result, err := loop.Run(ctx, "Run the mock tool")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result != "The result is: mock result" {
		t.Errorf("unexpected result: %s", result)
	}
	if callCount != 2 {
		t.Errorf("expected 2 LLM calls, got %d", callCount)
	}
}

func TestAgentLoop_StateTransitions(t *testing.T) {
	// Verify the state machine is used during the loop
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "done"),
			llm.FinishReasonStop,
		), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "test",
	})

	ctx := context.Background()
	_, err := loop.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify the loop used state transitions (not just set directly)
	transitions := loop.StateTransitions()
	if len(transitions) == 0 {
		t.Error("expected state transitions to be recorded")
	}

	// First transition should be Idle → Thinking
	if transitions[0].From != agent.AgentStateIdle || transitions[0].To != agent.AgentStateThinking {
		t.Errorf("first transition should be Idle → Thinking, got %s → %s",
			transitions[0].From, transitions[0].To)
	}

	// Last transition should end in Terminated
	last := transitions[len(transitions)-1]
	if last.To != agent.AgentStateTerminated {
		t.Errorf("last state should be Terminated, got %s", last.To)
	}
}

func TestAgentLoop_GovernanceDeny(t *testing.T) {
	// Governance blocks the action → loop should handle gracefully
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		msg := llm.Message{Role: llm.RoleAssistant, Content: "Let me run a dangerous command"}
		msg.WithToolCall("call-1", "shell", `{"command":"rm -rf /"}`)
		return llm.NewToolCallResponse("Let me run a dangerous command", msg.ToolCalls...), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	govCtx.Rules = []string{"SHELL-001"}
	toolReg := tools.NewRegistry()
	toolReg.Register(&mockTool{name: "shell", description: "shell tool", result: "ok"})

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "test",
	})

	ctx := context.Background()
	_, err := loop.Run(ctx, "Run rm -rf /")
	// Should error because governance denied the action
	if err == nil {
		t.Error("expected error when governance denies action")
	}
}

func TestAgentLoop_MaxIterations(t *testing.T) {
	// LLM keeps returning tool calls → loop should terminate after max iterations
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		msg := llm.Message{Role: llm.RoleAssistant, Content: "running tool..."}
		msg.WithToolCall("call-1", "mock", `{"input":"test"}`)
		return llm.NewToolCallResponse("running tool...", msg.ToolCalls...), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()
	toolReg.Register(&mockTool{name: "mock", description: "mock", result: "ok"})

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 3,
		SystemPrompt:  "test",
	})

	ctx := context.Background()
	_, err := loop.Run(ctx, "test")
	if err == nil {
		t.Error("expected error when max iterations exceeded")
	}

	// Verify 3 iterations happened
	if loop.iterations != 3 {
		t.Errorf("expected 3 iterations, got %d", loop.iterations)
	}
}

func TestDemoFeedbackLoop(t *testing.T) {
	// ★★★ KEY DEMO ★★★
	// Verifies the complete feedback loop:
	// MockProvider → Agent Loop → Tool Call → Governance → Observation → Message → MockProvider again
	//
	// First call: LLM returns a tool call
	// Second call: LLM sees the tool result in the message history

	callCount := 0
	var secondCallMessages []llm.Message

	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		callCount++

		if callCount == 1 {
			// First call: verify initial messages exist
			if len(messages) < 2 {
				t.Error("first call: expected at least system + user messages")
			}
			// Return a tool call
			msg := llm.Message{Role: llm.RoleAssistant, Content: "Let me check the date"}
			msg.WithToolCall("call-date", "mock", `{"input":"date"}`)
			return llm.NewToolCallResponse("Let me check the date", msg.ToolCalls...), nil
		}

		// Second call: should contain the tool result
		secondCallMessages = messages

		// Verify the message chain is complete
		hasToolMessage := false
		for _, m := range messages {
			if m.Role == llm.RoleTool && m.ToolCallID == "call-date" {
				hasToolMessage = true
				if m.Content == "" {
					t.Error("tool message should have content (the result)")
				}
				break
			}
		}
		if !hasToolMessage {
			t.Error("second call: expected tool result message in the conversation")
		}

		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "The current date is 2026-08-14"),
			llm.FinishReasonStop,
		), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()
	toolReg.Register(&mockTool{
		name:        "mock",
		description: "returns the current date",
		result:      "2026-08-14",
	})

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "You are a helpful assistant.",
	})

	ctx := context.Background()
	result, err := loop.Run(ctx, "What is today's date?")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify the final result
	if result != "The current date is 2026-08-14" {
		t.Errorf("unexpected final result: %s", result)
	}

	// Verify LLM was called exactly twice
	if callCount != 2 {
		t.Errorf("expected 2 LLM calls (tool call + final), got %d", callCount)
	}

	// Verify second call messages contain the complete conversation chain
	expectedRoles := []llm.Role{llm.RoleSystem, llm.RoleUser, llm.RoleAssistant, llm.RoleTool}
	if len(secondCallMessages) < len(expectedRoles) {
		t.Fatalf("expected at least %d messages in second call, got %d", len(expectedRoles), len(secondCallMessages))
	}
	for i, expected := range expectedRoles {
		if secondCallMessages[i].Role != expected {
			t.Errorf("message %d: expected role %s, got %s", i, expected, secondCallMessages[i].Role)
		}
	}

	t.Logf("✅ Feedback loop verified: %d LLM calls, %d messages in final call",
		callCount, len(secondCallMessages))
}

func TestAgentLoop_ToolNotFound(t *testing.T) {
	// LLM requests a tool that doesn't exist → loop should handle error gracefully
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		// Only one call since tool not found will add error tool message
		// and then the loop continues — but we need to handle this
		if len(messages) > 2 && messages[len(messages)-1].Role == llm.RoleTool {
			// This is the second call after tool error
			return llm.NewResponse(
				llm.NewMessage(llm.RoleAssistant, "I couldn't run that tool, let me try something else"),
				llm.FinishReasonStop,
			), nil
		}
		msg := llm.Message{Role: llm.RoleAssistant, Content: "Let me use a nonexistent tool"}
		msg.WithToolCall("call-1", "nonexistent", `{}`)
		return llm.NewToolCallResponse("Let me use a nonexistent tool", msg.ToolCalls...), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "test",
	})

	ctx := context.Background()
	result, err := loop.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	// Should recover gracefully
	if result == "" {
		t.Error("expected non-empty result despite tool error")
	}
}

func TestAgentLoop_ToolExecutionFailure(t *testing.T) {
	// Tool execution fails → loop should continue with error observation
	failingTool := &failingMockTool{}

	callCount := 0
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		callCount++
		if callCount == 1 {
			msg := llm.Message{Role: llm.RoleAssistant, Content: "Let me try something"}
			msg.WithToolCall("call-1", "failing", `{}`)
			return llm.NewToolCallResponse("Let me try something", msg.ToolCalls...), nil
		}
		// Second call: LLM sees the error and responds accordingly
		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "The tool failed, but I can still help"),
			llm.FinishReasonStop,
		), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()
	toolReg.Register(failingTool)

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "test",
	})

	ctx := context.Background()
	result, err := loop.Run(ctx, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result despite tool failure")
	}
}

// failingMockTool always fails on execute.
type failingMockTool struct{}

func (f *failingMockTool) Name() string        { return "failing" }
func (f *failingMockTool) Description() string  { return "always fails" }
func (f *failingMockTool) Execute(action protocol.Action) (protocol.ToolResult, error) {
	return protocol.NewErrorResult(action.ID, "simulated failure", 0), nil
}
func (f *failingMockTool) Validate(action protocol.Action) error { return nil }

func TestAgentLoop_ContextCancellation(t *testing.T) {
	// Context cancellation should terminate the loop
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		time.Sleep(100 * time.Millisecond)
		return llm.NewResponse(llm.NewMessage(llm.RoleAssistant, "ok"), llm.FinishReasonStop), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "test",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := loop.Run(ctx, "test")
	if err == nil {
		t.Error("expected error when context is cancelled")
	}
}

func TestAgentLoop_Messages(t *testing.T) {
	// Verify Messages() returns the full conversation history
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "done"),
			llm.FinishReasonStop,
		), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "You are helpful.",
	})

	_, err := loop.Run(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	msgs := loop.Messages()
	if len(msgs) < 3 {
		t.Errorf("expected at least 3 messages (system, user, assistant), got %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("first message should be system, got %s", msgs[0].Role)
	}
	if msgs[1].Role != llm.RoleUser {
		t.Errorf("second message should be user, got %s", msgs[1].Role)
	}
}

func TestAgentLoop_HITLApproval(t *testing.T) {
	// Shell command triggers HITL → approve → execute → complete
	callCount := 0
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		callCount++
		if callCount == 1 {
			msg := llm.Message{Role: llm.RoleAssistant, Content: "Let me run a shell command"}
			msg.WithToolCall("call-1", "shell", `{"command":"echo hello"}`)
			return llm.NewToolCallResponse("Let me run a shell command", msg.ToolCalls...), nil
		}
		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "Command executed successfully"),
			llm.FinishReasonStop,
		), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	govCtx.HITLTimeout = 300 * 1e9 // 300s
	toolReg := tools.NewRegistry()
	toolReg.Register(&mockTool{name: "shell", description: "shell", result: "hello"})

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "test",
	})

	// Before running, set up HITL approval callback
	var pendingAuth *governance.GovernanceAuth
	loop.SetHITLHandler(func(action protocol.Action, auth governance.GovernanceAuth) governance.GovernanceDecision {
		pendingAuth = &auth
		// Auto-approve for testing
		return governance.DecisionAllow
	})

	ctx := context.Background()
	result, err := loop.Run(ctx, "Run echo hello")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result != "Command executed successfully" {
		t.Errorf("unexpected result: %s", result)
	}
	if pendingAuth == nil {
		t.Error("expected HITL auth to be issued")
	}
}

func TestAgentLoop_ParseToolArguments(t *testing.T) {
	// Verify JSON arguments are parsed correctly
	args := `{"command":"echo hello","cwd":"/tmp"}`
	payload, err := parseToolArguments(args)
	if err != nil {
		t.Fatalf("parseToolArguments failed: %v", err)
	}
	if payload["command"] != "echo hello" {
		t.Errorf("expected command 'echo hello', got %v", payload["command"])
	}
	if payload["cwd"] != "/tmp" {
		t.Errorf("expected cwd '/tmp', got %v", payload["cwd"])
	}
}

func TestAgentLoop_ParseToolArguments_InvalidJSON(t *testing.T) {
	_, err := parseToolArguments("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestAgentLoop_RecordedMessages(t *testing.T) {
	// Verify the LLM provider receives the correct message sequence
	callCount := 0
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		callCount++
		if callCount == 1 {
			// Verify the initial messages
			if len(messages) != 2 {
				t.Errorf("first call: expected 2 messages (system + user), got %d", len(messages))
			}
			msg := llm.Message{Role: llm.RoleAssistant, Content: "running tool"}
			msg.WithToolCall("call-1", "mock", `{"input":"test"}`)
			return llm.NewToolCallResponse("running tool", msg.ToolCalls...), nil
		}
		// Second call: verify assistant + tool messages are present
		if len(messages) != 4 {
			t.Errorf("second call: expected 4 messages (system, user, assistant, tool), got %d", len(messages))
		}
		// Verify the tool message contains the result
		lastMsg := messages[len(messages)-1]
		if lastMsg.Role != llm.RoleTool {
			t.Errorf("last message should be tool, got %s", lastMsg.Role)
		}
		// Verify the tool message content is JSON-serialized ToolResult
		var result protocol.ToolResult
		if err := json.Unmarshal([]byte(lastMsg.Content), &result); err != nil {
			t.Errorf("tool message content should be valid JSON ToolResult: %v", err)
		}

		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "done"),
			llm.FinishReasonStop,
		), nil
	})

	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()
	toolReg.Register(&mockTool{name: "mock", description: "mock", result: "test result"})

	loop := NewAgentLoop(mock, governance.NewPipeline(govCtx), toolReg, LoopConfig{
		MaxIterations: 5,
		SystemPrompt:  "test",
	})

	_, err := loop.Run(context.Background(), "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}
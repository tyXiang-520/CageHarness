package llm

import (
	"context"
	"errors"
	"testing"
)

func TestProvider_InterfaceContract(t *testing.T) {
	// Verify that MockProvider implements Provider
	var p Provider = NewMockProvider(nil)
	if p == nil {
		t.Fatal("MockProvider should not be nil")
	}
}

func TestMockProvider_SequenceMode(t *testing.T) {
	responses := []Response{
		NewResponse(NewMessage(RoleAssistant, "first"), FinishReasonStop),
		NewResponse(NewMessage(RoleAssistant, "second"), FinishReasonStop),
	}
	p := NewMockProvider(responses)

	ctx := context.Background()

	resp, err := p.Generate(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content != "first" {
		t.Errorf("content = %q, want %q", resp.Message.Content, "first")
	}

	resp, err = p.Generate(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content != "second" {
		t.Errorf("content = %q, want %q", resp.Message.Content, "second")
	}
}

func TestMockProvider_SequenceModeExhausted(t *testing.T) {
	responses := []Response{
		NewResponse(NewMessage(RoleAssistant, "only"), FinishReasonStop),
	}
	p := NewMockProvider(responses)

	ctx := context.Background()
	p.Generate(ctx, nil) // consume the only response

	_, err := p.Generate(ctx, nil)
	if err == nil {
		t.Fatal("expected error when sequence exhausted")
	}
}

func TestMockProvider_FunctionMode(t *testing.T) {
	handler := func(messages []Message) (Response, error) {
		if len(messages) == 0 {
			return Response{}, errors.New("no messages")
		}
		return NewResponse(
			NewMessage(RoleAssistant, "echo: "+messages[0].Content),
			FinishReasonStop,
		), nil
	}
	p := NewMockProvider(nil)
	p.SetHandler(handler)

	ctx := context.Background()
	resp, err := p.Generate(ctx, []Message{NewMessage(RoleUser, "hello")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content != "echo: hello" {
		t.Errorf("content = %q, want %q", resp.Message.Content, "echo: hello")
	}
}

func TestMockProvider_FunctionModeError(t *testing.T) {
	handler := func(messages []Message) (Response, error) {
		return Response{}, errors.New("provider error")
	}
	p := NewMockProvider(nil)
	p.SetHandler(handler)

	ctx := context.Background()
	_, err := p.Generate(ctx, []Message{NewMessage(RoleUser, "hi")})
	if err == nil {
		t.Fatal("expected error from handler")
	}
}

func TestMockProvider_Reset(t *testing.T) {
	responses := []Response{
		NewResponse(NewMessage(RoleAssistant, "a"), FinishReasonStop),
		NewResponse(NewMessage(RoleAssistant, "b"), FinishReasonStop),
	}
	p := NewMockProvider(responses)

	ctx := context.Background()
	p.Generate(ctx, nil) // consume one
	p.Reset()

	// After reset, sequence should restart
	resp, err := p.Generate(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content != "a" {
		t.Errorf("content = %q, want %q", resp.Message.Content, "a")
	}
}

func TestMockProvider_ToolCallResponse(t *testing.T) {
	tc := ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: ToolCallFunction{
			Name:      "shell",
			Arguments: `{"cmd": "ls"}`,
		},
	}
	responses := []Response{
		NewToolCallResponse("", tc),
	}
	p := NewMockProvider(responses)

	ctx := context.Background()
	resp, err := p.Generate(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FinishReason != FinishReasonToolCalls {
		t.Errorf("FinishReason = %v, want %v", resp.FinishReason, FinishReasonToolCalls)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Function.Name != "shell" {
		t.Errorf("tool name = %q, want %q", resp.Message.ToolCalls[0].Function.Name, "shell")
	}
}

func TestMockProvider_RecordsMessages(t *testing.T) {
	p := NewMockProvider([]Response{
		NewResponse(NewMessage(RoleAssistant, "ok"), FinishReasonStop),
	})

	ctx := context.Background()
	messages := []Message{
		NewMessage(RoleUser, "hello"),
		NewMessage(RoleAssistant, "world"),
	}
	p.Generate(ctx, messages)

	recorded := p.Messages()
	if len(recorded) != 1 {
		t.Fatalf("expected 1 recorded call, got %d", len(recorded))
	}
	if len(recorded[0]) != 2 {
		t.Fatalf("expected 2 messages in first call, got %d", len(recorded[0]))
	}
	if recorded[0][0].Content != "hello" {
		t.Errorf("first message content = %q, want %q", recorded[0][0].Content, "hello")
	}
	if recorded[0][1].Content != "world" {
		t.Errorf("second message content = %q, want %q", recorded[0][1].Content, "world")
	}
}
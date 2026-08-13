package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAIProvider_InterfaceContract(t *testing.T) {
	// Verify OpenAIProvider implements Provider
	var p Provider = NewOpenAIProvider(OpenAIConfig{
		Endpoint: "https://api.openai.com/v1",
		Model:    "gpt-4o",
		APIKey:   "test-key",
	})
	if p == nil {
		t.Fatal("OpenAIProvider should not be nil")
	}
}

func TestOpenAIProvider_Config(t *testing.T) {
	cfg := OpenAIConfig{
		Endpoint:  "https://api.openai.com/v1",
		Model:     "gpt-4o",
		APIKey:    "sk-test123",
		MaxTokens: 2048,
		Timeout:   30 * time.Second,
	}
	p := NewOpenAIProvider(cfg)
	if p == nil {
		t.Fatal("provider should not be nil")
	}
}

func TestOpenAIProvider_SuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Authorization") != "Bearer sk-test123" {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), "Bearer sk-test123")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}

		// Verify request body
		var reqBody map[string]any
		json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["model"] != "gpt-4o" {
			t.Errorf("model = %v, want %v", reqBody["model"], "gpt-4o")
		}

		// Return a successful response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := `{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello! How can I help you?"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 5,
				"total_tokens": 15
			}
		}`
		w.Write([]byte(resp))
	}))
	defer server.Close()

	provider := NewOpenAIProvider(OpenAIConfig{
		Endpoint:  server.URL,
		Model:     "gpt-4o",
		APIKey:    "sk-test123",
		MaxTokens: 2048,
		Timeout:   5 * time.Second,
	})

	ctx := context.Background()
	resp, err := provider.Generate(ctx, []Message{
		NewMessage(RoleUser, "Hi"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content != "Hello! How can I help you?" {
		t.Errorf("content = %q, want %q", resp.Message.Content, "Hello! How can I help you?")
	}
	if resp.FinishReason != FinishReasonStop {
		t.Errorf("FinishReason = %v, want %v", resp.FinishReason, FinishReasonStop)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want %d", resp.Usage.TotalTokens, 15)
	}
}

func TestOpenAIProvider_ToolCallResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := `{
			"id": "chatcmpl-456",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call-1",
						"type": "function",
						"function": {
							"name": "shell",
							"arguments": "{\"cmd\": \"ls\"}"
						}
					}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {
				"prompt_tokens": 20,
				"completion_tokens": 10,
				"total_tokens": 30
			}
		}`
		w.Write([]byte(resp))
	}))
	defer server.Close()

	provider := NewOpenAIProvider(OpenAIConfig{
		Endpoint:  server.URL,
		Model:     "gpt-4o",
		APIKey:    "sk-test123",
		MaxTokens: 2048,
		Timeout:   5 * time.Second,
	})

	ctx := context.Background()
	resp, err := provider.Generate(ctx, []Message{NewMessage(RoleUser, "list files")})
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

func TestOpenAIProvider_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		resp := `{
			"error": {
				"message": "Rate limit exceeded",
				"type": "rate_limit_error",
				"code": "rate_limit"
			}
		}`
		w.Write([]byte(resp))
	}))
	defer server.Close()

	provider := NewOpenAIProvider(OpenAIConfig{
		Endpoint:  server.URL,
		Model:     "gpt-4o",
		APIKey:    "sk-test123",
		MaxTokens: 2048,
		Timeout:   5 * time.Second,
	})

	ctx := context.Background()
	_, err := provider.Generate(ctx, []Message{NewMessage(RoleUser, "test")})
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "rate_limit") && !strings.Contains(err.Error(), "429") {
		t.Errorf("error should mention rate limit, got: %v", err)
	}
}

func TestOpenAIProvider_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"1","object":"chat.completion","created":1,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider(OpenAIConfig{
		Endpoint:  server.URL,
		Model:     "gpt-4o",
		APIKey:    "sk-test123",
		MaxTokens: 2048,
		Timeout:   5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond) // ensure context is expired

	_, err := provider.Generate(ctx, []Message{NewMessage(RoleUser, "test")})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestOpenAIProvider_RequestValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  OpenAIConfig
	}{
		{"empty endpoint", OpenAIConfig{Endpoint: "", Model: "gpt-4o", APIKey: "key"}},
		{"empty model", OpenAIConfig{Endpoint: "https://api.openai.com/v1", Model: "", APIKey: "key"}},
		{"empty api key", OpenAIConfig{Endpoint: "https://api.openai.com/v1", Model: "gpt-4o", APIKey: ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAIProvider(tt.cfg)
			ctx := context.Background()
			_, err := provider.Generate(ctx, []Message{NewMessage(RoleUser, "test")})
			if err == nil {
				t.Error("expected validation error for missing config")
			}
		})
	}
}
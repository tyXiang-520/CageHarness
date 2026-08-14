package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIConfig configures the OpenAI-compatible provider.
type OpenAIConfig struct {
	Endpoint  string        `yaml:"endpoint"`
	Model     string        `yaml:"model"`
	APIKey    string        `yaml:"api_key"`
	MaxTokens int           `yaml:"max_tokens"`
	Timeout   time.Duration `yaml:"timeout"`
}

// ToolDefinition describes a tool for the LLM.
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema for the tool's parameters
}

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs.
type OpenAIProvider struct {
	config OpenAIConfig
	client *http.Client
	tools  []ToolDefinition
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(config OpenAIConfig) *OpenAIProvider {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &OpenAIProvider{
		config: config,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// SetTools configures the tool definitions sent to the LLM.
func (p *OpenAIProvider) SetTools(tools []ToolDefinition) {
	p.tools = tools
}

// openAIRequest is the request body for the OpenAI chat completions API.
type openAIRequest struct {
	Model     string          `json:"model"`
	Messages  []openAIMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens,omitempty"`
	Tools     []openAIToolDef `json:"tools,omitempty"`
	Stream    bool            `json:"stream,omitempty"`
}

// openAIToolDef is a tool definition in the OpenAI API format.
type openAIToolDef struct {
	Type     string                `json:"type"`
	Function openAIToolFunctionDef `json:"function"`
}

// openAIToolFunctionDef is the function definition for a tool.
type openAIToolFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  jsonParameters `json:"parameters"`
}

// jsonParameters is a JSON Schema object for tool parameters.
type jsonParameters struct {
	Type       string                  `json:"type"`
	Properties map[string]jsonProperty `json:"properties"`
	Required   []string                `json:"required,omitempty"`
}

// jsonProperty is a single property in a JSON Schema.
type jsonProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// openAIMessage is a message in the OpenAI API format.
type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openAIToolCallFunction `json:"function"`
}

type openAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openAIResponse is the response body from the OpenAI chat completions API.
type openAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage,omitempty"`
	Error   *openAIError   `json:"error,omitempty"`
}

type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// Generate implements the Provider interface.
func (p *OpenAIProvider) Generate(ctx context.Context, messages []Message) (Response, error) {
	// Validate config
	if p.config.Endpoint == "" {
		return Response{}, errors.New("openai provider: endpoint is required")
	}
	if p.config.Model == "" {
		return Response{}, errors.New("openai provider: model is required")
	}
	if p.config.APIKey == "" {
		return Response{}, errors.New("openai provider: API key is required")
	}

	// Convert internal messages to OpenAI format
	openAIMsgs := make([]openAIMessage, len(messages))
	for i, msg := range messages {
		m := openAIMessage{
			Role:    msg.Role.String(),
			Content: msg.Content,
		}
		if msg.Role == RoleTool {
			m.ToolCallID = msg.ToolCallID
		}
		if len(msg.ToolCalls) > 0 {
			m.ToolCalls = make([]openAIToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				m.ToolCalls[j] = openAIToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: openAIToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
		openAIMsgs[i] = m
	}

	// Build tool definitions
	var toolDefs []openAIToolDef
	for _, td := range p.tools {
		props := make(map[string]jsonProperty)
		var required []string
		if td.Parameters != nil {
			for k, v := range td.Parameters {
				propMap, ok := v.(map[string]any)
				if !ok {
					continue
				}
				prop := jsonProperty{}
				if propType, ok := propMap["type"].(string); ok {
					prop.Type = propType
				}
				if propDesc, ok := propMap["description"].(string); ok {
					prop.Description = propDesc
				}
				props[k] = prop

				// Check if this parameter is required
				if requiredVal, ok := propMap["required"].(bool); ok && requiredVal {
					required = append(required, k)
				}
			}
		}
		// Default: all parameters are required
		if len(required) == 0 && len(props) > 0 {
			for k := range props {
				required = append(required, k)
			}
		}

		toolDefs = append(toolDefs, openAIToolDef{
			Type: "function",
			Function: openAIToolFunctionDef{
				Name:        td.Name,
				Description: td.Description,
				Parameters: jsonParameters{
					Type:       "object",
					Properties: props,
					Required:   required,
				},
			},
		})
	}

	reqBody := openAIRequest{
		Model:     p.config.Model,
		Messages:  openAIMsgs,
		MaxTokens: p.config.MaxTokens,
		Tools:     toolDefs,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, fmt.Errorf("openai provider: marshal request: %w", err)
	}

	// Build URL
	url := strings.TrimRight(p.config.Endpoint, "/")
	if !strings.HasSuffix(url, "/chat/completions") {
		url += "/chat/completions"
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("openai provider: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("openai provider: do request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("openai provider: read response: %w", err)
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return Response{}, fmt.Errorf("openai provider: parse response: %w", err)
	}

	// Check for API error
	if httpResp.StatusCode != http.StatusOK {
		if apiResp.Error != nil {
			return Response{}, fmt.Errorf("openai provider: API error (status %d): %s (type: %s, code: %s)",
				httpResp.StatusCode, apiResp.Error.Message, apiResp.Error.Type, apiResp.Error.Code)
		}
		return Response{}, fmt.Errorf("openai provider: API error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	if len(apiResp.Choices) == 0 {
		return Response{}, errors.New("openai provider: no choices in response")
	}

	choice := apiResp.Choices[0]
	msg := choice.Message

	// Convert finish reason
	var finishReason FinishReason
	switch choice.FinishReason {
	case "stop":
		finishReason = FinishReasonStop
	case "tool_calls":
		finishReason = FinishReasonToolCalls
	case "length":
		finishReason = FinishReasonLength
	default:
		finishReason = FinishReasonError
	}

	// Convert to internal message
	internalMsg := Message{
		Role:    RoleAssistant,
		Content: msg.Content,
	}

	// Convert tool calls
	if len(msg.ToolCalls) > 0 {
		internalMsg.ToolCalls = make([]ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			internalMsg.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: ToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	// Build response
	resp := Response{
		Message:      internalMsg,
		FinishReason: finishReason,
	}

	if apiResp.Usage != nil {
		resp.Usage = Usage{
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
			TotalTokens:      apiResp.Usage.TotalTokens,
		}
	}

	return resp, nil
}

// Ensure OpenAIProvider implements Provider at compile time.
var _ Provider = (*OpenAIProvider)(nil)
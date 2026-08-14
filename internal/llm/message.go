package llm

import "fmt"

// Role represents the role of a message participant.
type Role int

const (
	RoleSystem Role = iota
	RoleUser
	RoleAssistant
	RoleTool
)

func (r Role) String() string {
	switch r {
	case RoleSystem:
		return "system"
	case RoleUser:
		return "user"
	case RoleAssistant:
		return "assistant"
	case RoleTool:
		return "tool"
	default:
		return fmt.Sprintf("Role(%d)", int(r))
	}
}

// FinishReason indicates why the LLM stopped generating.
type FinishReason int

const (
	FinishReasonStop FinishReason = iota
	FinishReasonToolCalls
	FinishReasonLength
	FinishReasonError
)

func (fr FinishReason) String() string {
	switch fr {
	case FinishReasonStop:
		return "stop"
	case FinishReasonToolCalls:
		return "tool_calls"
	case FinishReasonLength:
		return "length"
	case FinishReasonError:
		return "error"
	default:
		return fmt.Sprintf("FinishReason(%d)", int(fr))
	}
}

// ToolCallFunction represents a function call requested by the LLM.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (tcf ToolCallFunction) String() string {
	return fmt.Sprintf("%s(`%s`)", tcf.Name, tcf.Arguments)
}

// ToolCall represents a single tool call from the LLM.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// Message represents a single message in the LLM conversation.
type Message struct {
	Role      Role       `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string   `json:"tool_call_id,omitempty"`
}

// NewMessage creates a new message with the given role and content.
func NewMessage(role Role, content string) Message {
	return Message{Role: role, Content: content}
}

// NewSystemMessage creates a new system message.
func NewSystemMessage(content string) Message {
	return Message{Role: RoleSystem, Content: content}
}

// NewToolMessage creates a new tool message with the result of a tool call.
func NewToolMessage(toolCallID, content string) Message {
	return Message{Role: RoleTool, Content: content, ToolCallID: toolCallID}
}

// WithToolCall adds a tool call to the message.
func (m *Message) WithToolCall(id, name, arguments string) {
	if m.ToolCalls == nil {
		m.ToolCalls = make([]ToolCall, 0)
	}
	m.ToolCalls = append(m.ToolCalls, ToolCall{
		ID:   id,
		Type: "function",
		Function: ToolCallFunction{
			Name:      name,
			Arguments: arguments,
		},
	})
}

// Response represents a response from the LLM provider.
type Response struct {
	Message      Message      `json:"message"`
	FinishReason FinishReason `json:"finish_reason"`
	Usage        Usage        `json:"usage,omitempty"`
}

// Usage represents token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewResponse creates a new LLM response.
func NewResponse(message Message, finishReason FinishReason) Response {
	return Response{
		Message:      message,
		FinishReason: finishReason,
	}
}

// NewToolCallResponse creates a response with a tool call.
func NewToolCallResponse(content string, toolCalls ...ToolCall) Response {
	msg := Message{Role: RoleAssistant, Content: content, ToolCalls: toolCalls}
	return Response{
		Message:      msg,
		FinishReason: FinishReasonToolCalls,
	}
}
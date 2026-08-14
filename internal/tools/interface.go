package tools

import (
	"fmt"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// Tool is the interface that all tools must implement.
// Every tool invocation goes through Governance (not direct Execute call).
// The Agent NEVER directly invokes Tool.Execute() — all execution must pass
// through the Governance pipeline.
type Tool interface {
	// Name returns the unique identifier for this tool (e.g. "shell", "file_read").
	Name() string
	// Description returns a human-readable description for LLM context.
	Description() string
	// Execute performs the tool's action and returns the result.
	// This must only be called by the Governance layer, never by the Agent directly.
	Execute(action protocol.Action) (ToolResult, error)
	// Validate checks whether the given action is valid for this tool.
	// Returns nil if valid, an error describing the issue otherwise.
	Validate(action protocol.Action) error
}

// Registry manages the set of available tools.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
// Returns an error if a tool with the same name is already registered.
func (r *Registry) Register(t Tool) error {
	name := t.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}
	r.tools[name] = t
	return nil
}

// Get retrieves a tool by name.
// Returns the tool and true if found, zero value and false otherwise.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools.
func (r *Registry) List() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}
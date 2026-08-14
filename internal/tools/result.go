package tools

import (
	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// ToolResult is a type alias for protocol.ToolResult.
// This avoids circular imports between tools and agent packages.
type ToolResult = protocol.ToolResult

// Re-exported functions for backward compatibility.
var (
	NewSuccessResult = protocol.NewSuccessResult
	NewErrorResult   = protocol.NewErrorResult
)
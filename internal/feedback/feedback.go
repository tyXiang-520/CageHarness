package feedback

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/agent"
	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

// TestFailureDetail captures a single test failure.
type TestFailureDetail struct {
	TestName string `json:"test_name"`
	Package  string `json:"package,omitempty"`
	Message  string `json:"message,omitempty"`
}

// FeedbackObservation is a structured analysis of a tool execution result.
// It wraps the raw ToolResult with parsed, LLM-friendly information.
type FeedbackObservation struct {
	success  bool
	source   string
	output   string
	errorMsg string
	failures []TestFailureDetail
}

// IsSuccess returns true if the tool execution was successful.
func (o FeedbackObservation) IsSuccess() bool {
	return o.success
}

// Source returns the tool type that produced this observation.
func (o FeedbackObservation) Source() string {
	return o.source
}

// Failures returns test failure details (for test tools).
func (o FeedbackObservation) Failures() []TestFailureDetail {
	return o.failures
}

// IsError returns true if this observation represents an error.
func (o FeedbackObservation) IsError() bool {
	return !o.success
}

// Summary returns a one-line summary of the observation.
func (o FeedbackObservation) Summary() string {
	if o.success {
		return fmt.Sprintf("[%s] success: %s", o.source, truncate(o.output, 80))
	}
	return fmt.Sprintf("[%s] error: %s", o.source, truncate(o.errorMsg, 80))
}

// FormatForLLM returns a formatted string suitable for LLM context.
func (o FeedbackObservation) FormatForLLM() string {
	var b strings.Builder
	if o.success {
		b.WriteString("Tool execution succeeded.\n")
	} else {
		b.WriteString("Tool execution failed.\n")
		if o.errorMsg != "" {
			b.WriteString(fmt.Sprintf("Error: %s\n", o.errorMsg))
		}
	}

	if o.output != "" {
		b.WriteString(fmt.Sprintf("Output:\n%s\n", o.output))
	}

	if len(o.failures) > 0 {
		b.WriteString(fmt.Sprintf("Test failures (%d):\n", len(o.failures)))
		for _, f := range o.failures {
			b.WriteString(fmt.Sprintf("  - %s", f.TestName))
			if f.Message != "" {
				b.WriteString(fmt.Sprintf(": %s", f.Message))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// ToObservation converts to an agent.Observation for state tracking.
func (o FeedbackObservation) ToObservation() agent.Observation {
	return agent.Observation{
		ActionID: "", // Set by caller
		Result: protocol.ToolResult{
			Success: o.success,
			Data:    o.output,
			Error:   o.errorMsg,
		},
		Timestamp: time.Now(),
	}
}

// FeedbackProcessor processes tool results into structured observations.
type FeedbackProcessor struct{}

// NewFeedbackProcessor creates a new FeedbackProcessor.
func NewFeedbackProcessor() *FeedbackProcessor {
	return &FeedbackProcessor{}
}

// Process analyzes a tool result and produces a FeedbackObservation.
func (fp *FeedbackProcessor) Process(toolType string, result protocol.ToolResult) FeedbackObservation {
	output := dataToString(result.Data)
	switch toolType {
	case "shell", "execute_shell":
		return ParseShellResult(output, result.Error, result.Success)
	case "run_tests", "test":
		return ParseTestOutput(output, result.Error, result.Success)
	default:
		return genericObservation(toolType, result)
	}
}

// ParseShellResult parses a shell command execution result.
func ParseShellResult(stdout, stderr string, success bool) FeedbackObservation {
	errorMsg := stderr
	if !success && stderr == "" {
		errorMsg = "command failed"
	}

	return FeedbackObservation{
		success:  success,
		source:   "shell",
		output:   stdout,
		errorMsg: errorMsg,
	}
}

// testEvent is a single line from `go test -json` output.
type testEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
}

// ParseTestOutput parses `go test -json` output into a structured observation.
func ParseTestOutput(stdout, stderr string, success bool) FeedbackObservation {
	if stdout == "" && success {
		return FeedbackObservation{
			success: true,
			source:  "go_test",
		}
	}

	// Parse JSON lines
	lines := strings.Split(stdout, "\n")
	failedTests := make(map[string]TestFailureDetail)
	overallPass := success

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var evt testEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			// Non-JSON line — check for failure indicators
			if strings.Contains(line, "FAIL") {
				overallPass = false
			}
			continue
		}

		switch evt.Action {
		case "fail":
			if evt.Test != "" {
				failedTests[evt.Test] = TestFailureDetail{
					TestName: evt.Test,
					Package:  evt.Package,
				}
			} else {
				overallPass = false
			}
		case "output":
			if strings.Contains(evt.Output, "--- FAIL:") {
				overallPass = false
			}
		}
	}

	// Non-JSON output fallback
	if len(failedTests) == 0 && overallPass {
		if strings.Contains(stdout, "FAIL") {
			overallPass = false
		}
	}

	failures := make([]TestFailureDetail, 0, len(failedTests))
	for _, f := range failedTests {
		failures = append(failures, f)
	}

	errorMsg := stderr
	if !overallPass && errorMsg == "" {
		errorMsg = fmt.Sprintf("%d test(s) failed", len(failures))
	}

	return FeedbackObservation{
		success:  overallPass,
		source:   "go_test",
		output:   stdout,
		errorMsg: errorMsg,
		failures: failures,
	}
}

// genericObservation creates a generic observation for unknown tool types.
func genericObservation(toolType string, result protocol.ToolResult) FeedbackObservation {
	return FeedbackObservation{
		success:  result.Success,
		source:   toolType,
		output:   dataToString(result.Data),
		errorMsg: result.Error,
	}
}

// dataToString converts the Data field of a ToolResult to a string.
func dataToString(data any) string {
	if data == nil {
		return ""
	}
	switch v := data.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
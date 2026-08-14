package feedback

import (
	"testing"

	"github.com/tyXiang-520/CageHarness/internal/protocol"
)

func TestParseTestOutput_Pass(t *testing.T) {
	// Simulate `go test -json ./...` output for a passing test
	stdout := `{"Time":"2026-08-14T10:00:00Z","Action":"run","Package":"example","Test":"TestFoo"}
{"Time":"2026-08-14T10:00:01Z","Action":"output","Package":"example","Test":"TestFoo","Output":"=== RUN   TestFoo\n"}
{"Time":"2026-08-14T10:00:01Z","Action":"output","Package":"example","Test":"TestFoo","Output":"--- PASS: TestFoo (0.00s)\n"}
{"Time":"2026-08-14T10:00:01Z","Action":"pass","Package":"example","Test":"TestFoo"}
{"Time":"2026-08-14T10:00:01Z","Action":"output","Package":"example","Output":"ok  \texample\t0.123s\n"}
{"Time":"2026-08-14T10:00:01Z","Action":"pass","Package":"example"}`

	obs := ParseTestOutput(stdout, "", true)
	if !obs.IsSuccess() {
		t.Error("expected success for passing tests")
	}
	if obs.Source() != "go_test" {
		t.Errorf("expected source go_test, got %s", obs.Source())
	}
}

func TestParseTestOutput_Fail(t *testing.T) {
	stdout := `{"Time":"2026-08-14T10:00:00Z","Action":"run","Package":"example","Test":"TestBar"}
{"Time":"2026-08-14T10:00:01Z","Action":"output","Package":"example","Test":"TestBar","Output":"=== RUN   TestBar\n"}
{"Time":"2026-08-14T10:00:01Z","Action":"output","Package":"example","Test":"TestBar","Output":"--- FAIL: TestBar (0.00s)\n"}
{"Time":"2026-08-14T10:00:01Z","Action":"output","Package":"example","Test":"TestBar","Output":"    test_test.go:10: expected 1, got 2\n"}
{"Time":"2026-08-14T10:00:01Z","Action":"fail","Package":"example","Test":"TestBar"}
{"Time":"2026-08-14T10:00:01Z","Action":"output","Package":"example","Output":"FAIL\texample\t0.123s\n"}
{"Time":"2026-08-14T10:00:01Z","Action":"fail","Package":"example"}`

	obs := ParseTestOutput(stdout, "", false)
	if obs.IsSuccess() {
		t.Error("expected failure for failing tests")
	}
	if obs.Source() != "go_test" {
		t.Errorf("expected source go_test, got %s", obs.Source())
	}

	// Verify failure details
	failures := obs.Failures()
	if len(failures) == 0 {
		t.Error("expected failure details")
	}
	if len(failures) > 0 && failures[0].TestName != "TestBar" {
		t.Errorf("expected TestBar failure, got %s", failures[0].TestName)
	}
}

func TestParseTestOutput_EmptyOutput(t *testing.T) {
	obs := ParseTestOutput("", "", true)
	if !obs.IsSuccess() {
		t.Error("empty output should be success")
	}
}

func TestParseTestOutput_NonJSONOutput(t *testing.T) {
	// Raw text output (non-JSON) should still be parsed
	stdout := "ok  \texample\t0.123s\n"
	obs := ParseTestOutput(stdout, "", true)
	if !obs.IsSuccess() {
		t.Error("non-JSON pass output should be success")
	}
}

func TestParseShellResult_Success(t *testing.T) {
	obs := ParseShellResult("hello world\n", "", true)
	if !obs.IsSuccess() {
		t.Error("expected success with stdout")
	}
	if obs.Source() != "shell" {
		t.Errorf("expected source shell, got %s", obs.Source())
	}
}

func TestParseShellResult_Error(t *testing.T) {
	obs := ParseShellResult("", "command not found: xyz\n", false)
	if obs.IsSuccess() {
		t.Error("expected failure for non-zero exit code")
	}
	if obs.Source() != "shell" {
		t.Errorf("expected source shell, got %s", obs.Source())
	}
}

func TestParseShellResult_ExitCode1(t *testing.T) {
	obs := ParseShellResult("", "error: something went wrong", false)
	if obs.IsSuccess() {
		t.Error("expected failure for failed execution")
	}
}

func TestParseShellResult_StderrButExitZero(t *testing.T) {
	// Some tools write to stderr but succeed (warnings, etc.)
	obs := ParseShellResult("output", "warning: deprecated", true)
	if !obs.IsSuccess() {
		t.Error("success should be success even with stderr")
	}
}

func TestParseShellResult_EmptyOutput(t *testing.T) {
	obs := ParseShellResult("", "", true)
	if !obs.IsSuccess() {
		t.Error("empty output with success should be success")
	}
}

func TestObservation_Serialization(t *testing.T) {
	// Verify Observation can be serialized for LLM context
	obs := ParseShellResult("hello", "", true)
	text := obs.Summary()
	if text == "" {
		t.Error("summary should not be empty")
	}
	if !obs.IsSuccess() {
		t.Error("successful shell should be success")
	}
}

func TestFeedbackProcessor_Process(t *testing.T) {
	fp := NewFeedbackProcessor()

	t.Run("shell result", func(t *testing.T) {
		result := protocol.NewSuccessResult("act-1", "hello", 0)
		obs := fp.Process("shell", result)
		if obs.Source() != "shell" {
			t.Errorf("expected source shell, got %s", obs.Source())
		}
		if !obs.IsSuccess() {
			t.Error("expected success")
		}
	})

	t.Run("test result", func(t *testing.T) {
		result := protocol.NewSuccessResult("act-2", "ok\texample\t0.123s", 0)
		obs := fp.Process("run_tests", result)
		if obs.Source() != "go_test" {
			t.Errorf("expected source go_test, got %s", obs.Source())
		}
	})

	t.Run("unknown tool type", func(t *testing.T) {
		result := protocol.NewSuccessResult("act-3", "data", 0)
		obs := fp.Process("unknown_tool", result)
		// Should still produce a valid observation
		if obs.Source() != "unknown_tool" {
			t.Errorf("expected source unknown_tool, got %s", obs.Source())
		}
	})
}

func TestObservation_ToToolMessage(t *testing.T) {
	// Verify Observation can be formatted for LLM tool message
	result := protocol.NewSuccessResult("act-1", "hello world", 0)
	fp := NewFeedbackProcessor()
	obs := fp.Process("shell", result)

	content := obs.FormatForLLM()
	if content == "" {
		t.Error("FormatForLLM should not return empty")
	}

	// Verify it contains the actual output
	if !containsSubstring(content, "hello world") {
		t.Errorf("expected output in formatted content, got: %s", content)
	}
}

// Helper: check if Observation carries failure info
func TestObservation_TestFailureDetails(t *testing.T) {
	stdout := `{"Time":"2026-08-14T10:00:00Z","Action":"run","Package":"pkg","Test":"TestX"}
{"Time":"2026-08-14T10:00:01Z","Action":"output","Package":"pkg","Test":"TestX","Output":"--- FAIL: TestX (0.00s)\n"}
{"Time":"2026-08-14T10:00:01Z","Action":"fail","Package":"pkg","Test":"TestX"}
{"Time":"2026-08-14T10:00:01Z","Action":"fail","Package":"pkg"}`

	obs := ParseTestOutput(stdout, "assertion error: want 1 got 2", false)
	if obs.IsSuccess() {
		t.Error("expected failure")
	}

	failures := obs.Failures()
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}
	if failures[0].TestName != "TestX" {
		t.Errorf("expected TestX, got %s", failures[0].TestName)
	}
}

// Helper: verify FeedbackObservation type
func TestFeedbackObservation_IsError(t *testing.T) {
	successObs := ParseShellResult("ok", "", true)
	if successObs.IsError() {
		t.Error("success observation should not be error")
	}

	errorObs := ParseShellResult("", "fail", false)
	if !errorObs.IsError() {
		t.Error("error observation should be error")
	}
}

func TestObservation_ToObservation(t *testing.T) {
	// Verify FeedbackObservation converts to agent.Observation
	result := protocol.NewSuccessResult("act-1", "hello", 0)
	fp := NewFeedbackProcessor()
	fObs := fp.Process("shell", result)

	obs := fObs.ToObservation()
	if obs.Timestamp.IsZero() {
		t.Error("observation should have timestamp")
	}
}

func TestObservation_FormatForLLM_IncludesSuccess(t *testing.T) {
	result := protocol.NewSuccessResult("act-1", "build successful", 0)
	fp := NewFeedbackProcessor()
	obs := fp.Process("shell", result)

	content := obs.FormatForLLM()
	if !containsSubstring(content, "build successful") {
		t.Errorf("expected output content in formatted message, got: %s", content)
	}
}

func TestObservation_FormatForLLM_IncludesError(t *testing.T) {
	result := protocol.NewErrorResult("act-1", "command not found", 0)
	fp := NewFeedbackProcessor()
	obs := fp.Process("shell", result)

	content := obs.FormatForLLM()
	if !containsSubstring(content, "command not found") {
		t.Errorf("expected error in formatted message, got: %s", content)
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Verify agent.Observation type is used correctly
func TestObservation_WrapsAgentObservation(t *testing.T) {
	result := protocol.NewSuccessResult("act-1", "data", 0)
	fp := NewFeedbackProcessor()
	fObs := fp.Process("file_read", result)

	obs := fObs.ToObservation()
	if obs.Result.Success != true {
		t.Error("expected successful result")
	}
}

// Verify the FeedbackProcessor handles nil results gracefully
func TestFeedbackProcessor_NilResult(t *testing.T) {
	fp := NewFeedbackProcessor()
	obs := fp.Process("shell", protocol.ToolResult{})
	if obs.IsSuccess() {
		t.Error("empty result should not be success")
	}
}

func TestObservation_Summary(t *testing.T) {
	tests := []struct {
		name string
		obs  FeedbackObservation
	}{
		{
			name: "success shell",
			obs:  ParseShellResult("hello world", "", true),
		},
		{
			name: "error shell",
			obs:  ParseShellResult("", "permission denied", false),
		},
		{
			name: "pass test",
			obs:  ParseTestOutput("ok\texample\t0.1s", "", true),
		},
		{
			name: "fail test",
			obs:  ParseTestOutput("FAIL\texample\t0.1s", "test failed", false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := tt.obs.Summary()
			if len(summary) == 0 {
				t.Error("summary should not be empty")
			}
		})
	}
}

// Verify multiple test failures are captured
func TestParseTestOutput_MultipleFailures(t *testing.T) {
	stdout := `{"Time":"2026-08-14T10:00:00Z","Action":"run","Package":"pkg","Test":"TestA"}
{"Time":"2026-08-14T10:00:01Z","Action":"output","Package":"pkg","Test":"TestA","Output":"--- FAIL: TestA\n"}
{"Time":"2026-08-14T10:00:01Z","Action":"fail","Package":"pkg","Test":"TestA"}
{"Time":"2026-08-14T10:00:01Z","Action":"run","Package":"pkg","Test":"TestB"}
{"Time":"2026-08-14T10:00:01Z","Action":"output","Package":"pkg","Test":"TestB","Output":"--- FAIL: TestB\n"}
{"Time":"2026-08-14T10:00:01Z","Action":"fail","Package":"pkg","Test":"TestB"}
{"Time":"2026-08-14T10:00:01Z","Action":"fail","Package":"pkg"}`

	obs := ParseTestOutput(stdout, "", false)
	failures := obs.Failures()
	if len(failures) != 2 {
		t.Errorf("expected 2 failures, got %d", len(failures))
	}
}

func TestDataToString(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		expected string
	}{
		{"string", "hello", "hello"},
		{"bytes", []byte("world"), "world"},
		{"int", 42, "42"},
		{"nil", nil, ""},
		{"float", 3.14, "3.14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dataToString(tt.data)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
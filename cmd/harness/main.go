package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tyXiang-520/CageHarness/internal/cli"
	"github.com/tyXiang-520/CageHarness/internal/governance"
	"github.com/tyXiang-520/CageHarness/internal/llm"
	"github.com/tyXiang-520/CageHarness/internal/runtime"
	"github.com/tyXiang-520/CageHarness/internal/tools"
	"github.com/tyXiang-520/CageHarness/internal/web"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Wire up dependencies
	// TODO: Phase 11 — replace with real LLM provider and tools
	llmProvider := llm.NewMockProvider(nil)
	llmProvider.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "Task completed."),
			llm.FinishReasonStop,
		), nil
	})
	govCtx := governance.DefaultGovernanceContext()
	toolReg := tools.NewRegistry()
	tm := runtime.NewTaskManager()

	loop := runtime.NewAgentLoop(llmProvider, governance.NewPipeline(govCtx), toolReg, runtime.DefaultLoopConfig())
	c := cli.NewCLI(tm, loop)

	ctx := context.Background()

	switch os.Args[1] {
	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: harness run <task>")
			os.Exit(1)
		}
		task := os.Args[2]
		result, err := c.Run(ctx, task)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)

	case "submit":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: harness submit <task>")
			os.Exit(1)
		}
		taskID := c.Submit(ctx, os.Args[2])
		fmt.Printf("Task submitted: %s\n", taskID)

	case "status":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: harness status <task-id>")
			os.Exit(1)
		}
		task, err := c.Status(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("ID:      %s\n", task.ID)
		fmt.Printf("Task:    %s\n", task.Task)
		fmt.Printf("Status:  %s\n", task.Status)
		if task.Result != "" {
			fmt.Printf("Result:  %s\n", task.Result)
		}
		if task.Error != "" {
			fmt.Printf("Error:   %s\n", task.Error)
		}

	case "list":
		tasks := c.List()
		if len(tasks) == 0 {
			fmt.Println("No tasks.")
			return
		}
		for _, task := range tasks {
			fmt.Printf("%s  %-12s  %s\n", task.ID, task.Status, task.Task)
		}

	case "cancel":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: harness cancel <task-id>")
			os.Exit(1)
		}
		if err := c.Cancel(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Task %s cancelled.\n", os.Args[2])

	case "serve":
		port := "8080"
		if len(os.Args) >= 3 {
			port = os.Args[2]
		}
		addr := ":" + port
		srv := web.NewServer(tm, loop)
		fmt.Printf("\n  CageHarness WebUI starting at http://localhost%s\n\n", addr)
		fmt.Println("  Governance Pipeline: Schema → Risk → Policy → Boundary → Control")
		fmt.Println("  Press Ctrl+C to stop")
		if err := srv.Start(addr); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`CageHarness — AI4SE Coding Agent Harness

Usage:
  harness run     <task>       Run a task synchronously
  harness submit  <task>       Submit a task asynchronously
  harness status  <task-id>    Check task status
  harness list                 List all tasks
  harness cancel  <task-id>    Cancel a running task
  harness serve   [port]       Start WebUI server (default :8080)`)
}
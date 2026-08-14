package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/tyXiang-520/CageHarness/internal/cli"
	"github.com/tyXiang-520/CageHarness/internal/credential"
	"github.com/tyXiang-520/CageHarness/internal/governance"
	"github.com/tyXiang-520/CageHarness/internal/llm"
	"github.com/tyXiang-520/CageHarness/internal/runtime"
	"github.com/tyXiang-520/CageHarness/internal/tools"
	"github.com/tyXiang-520/CageHarness/internal/web"
)

// defaultCredentialNames lists the credential names that the harness manages.
var defaultCredentialNames = []string{
	"OPENAI_API_KEY",
	"ANTHROPIC_API_KEY",
}

func main() {
	// Create tool registry and register tools
	toolReg := tools.NewRegistry()
	registerTools(toolReg)

	// Resolve LLM provider: try real OpenAI API first, fall back to mock
	llmProvider := resolveLLMProvider(toolReg)

	govCtx := governance.DefaultGovernanceContext()
	tm := runtime.NewTaskManager()

	loop := runtime.NewAgentLoop(llmProvider, governance.NewPipeline(govCtx), toolReg, runtime.DefaultLoopConfig())
	c := cli.NewCLI(tm, loop)

	ctx := context.Background()

	// SCF / cloud hosting: default to serve mode when no arguments provided
	cmd := ""
	if len(os.Args) >= 2 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "", "serve":
		port := "8080"
		// SCF / cloud hosting: respect PORT environment variable
		if envPort := os.Getenv("PORT"); envPort != "" {
			port = envPort
		}
		// Alibaba Cloud FC: respect FC_SERVER_PORT
		if envPort := os.Getenv("FC_SERVER_PORT"); envPort != "" {
			port = envPort
		}
		// CLI argument overrides both default and env
		if len(os.Args) >= 3 {
			port = os.Args[2]
		}
		addr := ":" + port
		srv := web.NewServer(tm, loop)
		fmt.Printf("\n  CageHarness WebUI starting at http://0.0.0.0%s\n\n", addr)
		fmt.Println("  Governance Pipeline: Schema → Risk → Policy → Boundary → Control")
		fmt.Println("  Press Ctrl+C to stop")
		if err := srv.Start(addr); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}

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

	case "key":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: harness key <setup|status|clear> [name]")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "  setup       Interactive guided entry for an API key")
			fmt.Fprintln(os.Stderr, "  status      Show configured keys (never shows plaintext)")
			fmt.Fprintln(os.Stderr, "  clear       Remove a stored key")
			os.Exit(1)
		}
		handleKeyCommand(os.Args[2:])

	default:
		printUsage()
		os.Exit(1)
	}
}

// registerTools registers all available tools in the registry.
func registerTools(reg *tools.Registry) {
	// Shell tool — executes shell commands
	shellTool := tools.NewShellTool(tools.ShellConfig{Timeout: 30 * time.Second})
	reg.Register(shellTool)

	// File tool — registered as two separate tools: file_read and file_write
	fileTool := tools.NewFileTool(tools.FileConfig{WorkspaceRoot: "."})
	reg.Register(&tools.NamedTool{Tool: fileTool, NameOverride: "file_read"})
	reg.Register(&tools.NamedTool{Tool: fileTool, NameOverride: "file_write"})
}

// buildToolDefinitions converts registered tools to LLM tool definitions
// with proper JSON Schema parameters for the OpenAI function-calling API.
func buildToolDefinitions(reg *tools.Registry) []llm.ToolDefinition {
	defs := make([]llm.ToolDefinition, 0)

	for _, t := range reg.List() {
		var params map[string]any
		switch t.Name() {
		case "shell":
			params = map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The shell command to execute (e.g., 'go build ./...', 'ls -la')",
					"required":    true,
				},
			}
		case "file_read":
			params = map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to read, relative to workspace root",
					"required":    true,
				},
			}
		case "file_write":
			params = map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to write, relative to workspace root",
					"required":    true,
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The content to write to the file",
					"required":    true,
				},
			}
		default:
			// Generic fallback — empty parameters
			params = map[string]any{}
		}

		defs = append(defs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  params,
		})
	}

	return defs
}

// handleKeyCommand processes the 'harness key' subcommands.
func handleKeyCommand(args []string) {
	// Resolve the credential store chain:
	// 1. Try OS keychain first (most secure)
	// 2. Fall back to environment variables / .env file
	keychain := credential.NewKeychainStore()
	envStore := credential.NewEnvStore()

	// Try to load .env file if it exists
	if _, err := os.Stat(".env"); err == nil {
		if err := envStore.LoadDotEnv(".env"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load .env: %v\n", err)
		}
	}

	// Pick the best available store
	var store credential.CredentialStore
	var storeLabel string
	if keychain.IsAvailable() {
		store = keychain
		storeLabel = "OS keychain"
	} else {
		store = envStore
		storeLabel = "environment variables"
	}

	switch args[0] {
	case "setup":
		handleKeySetup(store, storeLabel, args[1:])

	case "status":
		handleKeyStatus(store, envStore, storeLabel, args[1:])

	case "clear":
		handleKeyClear(store, storeLabel, args[1:])

	default:
		fmt.Fprintf(os.Stderr, "unknown key command: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "usage: harness key <setup|status|clear> [name]")
		os.Exit(1)
	}
}

// handleKeySetup guides the user through securely entering an API key.
func handleKeySetup(store credential.CredentialStore, storeLabel string, args []string) {
	credName := "OPENAI_API_KEY"
	if len(args) >= 1 {
		credName = args[0]
	}

	fmt.Printf("\n  🛡️  CageHarness Credential Setup\n")
	fmt.Printf("  Storage: %s\n", storeLabel)
	fmt.Printf("  Credential: %s\n\n", credName)

	// Check if already set
	if store.Exists(credName) {
		fmt.Printf("  ⚠️  %s is already configured.\n", credName)
		fmt.Print("  Overwrite? [y/N]: ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" && answer != "yes" {
			fmt.Println("  Cancelled.")
			return
		}
	}

	fmt.Printf("  Enter value for %s:\n", credName)
	fmt.Println("  (input will be hidden — press Enter when done, Ctrl+C to cancel)")

	secret, err := credential.ReadPassword("  > ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n  error: %v\n", err)
		os.Exit(1)
	}

	if secret == "" {
		fmt.Println("\n  error: empty key not allowed")
		os.Exit(1)
	}

	if err := store.Set(credName, secret); err != nil {
		fmt.Fprintf(os.Stderr, "\n  error: failed to store credential: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  ✅ %s stored successfully in %s.\n", credName, storeLabel)
	fmt.Printf("  Verify with: harness key status\n")
}

// handleKeyStatus shows configured keys without revealing plaintext.
func handleKeyStatus(store credential.CredentialStore, envStore *credential.EnvStore, storeLabel string, args []string) {
	fmt.Printf("\n  🔑  CageHarness Credential Status\n")
	fmt.Printf("  Primary storage: %s\n\n", storeLabel)

	// Determine which names to check
	names := defaultCredentialNames
	if len(args) >= 1 {
		names = []string{args[0]}
	}

	foundAny := false
	for _, name := range names {
		// Check primary store
		if store.Exists(name) {
			val, err := store.Get(name)
			masked := credential.MaskKey(val)
			if err != nil {
				masked = "(error reading)"
			}
			fmt.Printf("  %-25s %s  [%s]\n", name, masked, storeLabel)
			foundAny = true
		} else if envStore != nil && envStore.Exists(name) {
			// Check env store as fallback
			val, err := envStore.Get(name)
			masked := credential.MaskKey(val)
			if err != nil {
				masked = "(error reading)"
			}
			fmt.Printf("  %-25s %s  [environment]\n", name, masked)
			foundAny = true
		} else {
			fmt.Printf("  %-25s (not set)\n", name)
		}
	}

	if !foundAny {
		fmt.Println("\n  No credentials configured.")
		fmt.Println("  Run 'harness key setup' to configure your API keys.")
	}
	fmt.Println()
}

// handleKeyClear removes a stored key.
func handleKeyClear(store credential.CredentialStore, storeLabel string, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: harness key clear <name>")
		fmt.Fprintln(os.Stderr, "  e.g. harness key clear OPENAI_API_KEY")
		os.Exit(1)
	}

	credName := args[0]

	if !store.Exists(credName) {
		fmt.Fprintf(os.Stderr, "  %s is not set in %s.\n", credName, storeLabel)
		os.Exit(1)
	}

	fmt.Printf("  ⚠️  This will remove %s from %s.\n", credName, storeLabel)
	fmt.Print("  Are you sure? [y/N]: ")
	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" && answer != "yes" {
		fmt.Println("  Cancelled.")
		return
	}

	if err := store.Delete(credName); err != nil {
		fmt.Fprintf(os.Stderr, "  error: failed to delete credential: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  ✅ %s removed from %s.\n", credName, storeLabel)
}

// resolveLLMProvider tries to create a real OpenAI provider using the stored API key.
// Falls back to MockProvider if no API key is configured.
func resolveLLMProvider(toolReg *tools.Registry) llm.Provider {
	// Try to get API key from credential stores
	keychain := credential.NewKeychainStore()
	envStore := credential.NewEnvStore()

	// Load .env if present
	if _, err := os.Stat(".env"); err == nil {
		envStore.LoadDotEnv(".env")
	}

	apiKey := ""
	switch {
	case keychain.IsAvailable() && keychain.Exists("OPENAI_API_KEY"):
		if v, err := keychain.Get("OPENAI_API_KEY"); err == nil {
			apiKey = v
		}
	case envStore.Exists("OPENAI_API_KEY"):
		if v, err := envStore.Get("OPENAI_API_KEY"); err == nil {
			apiKey = v
		}
	}

	if apiKey != "" {
		endpoint := os.Getenv("OPENAI_ENDPOINT")
		if endpoint == "" {
			endpoint = "https://api.openai.com/v1"
		}
		model := os.Getenv("OPENAI_MODEL")
		if model == "" {
			model = "gpt-3.5-turbo"
		}

		fmt.Fprintf(os.Stderr, "  Using OpenAI provider: model=%s endpoint=%s\n", model, endpoint)
		provider := llm.NewOpenAIProvider(llm.OpenAIConfig{
			Endpoint:  endpoint,
			Model:     model,
			APIKey:    apiKey,
			MaxTokens: 4096,
		})

		// Wire tool definitions to the LLM provider so the model knows
		// what tools are available for function calling.
		provider.SetTools(buildToolDefinitions(toolReg))

		return provider
	}

	// Fall back to mock provider
	fmt.Fprintln(os.Stderr, "  ⚠️  No API key configured. Using mock provider (run 'harness key setup' to configure)")
	mock := llm.NewMockProvider(nil)
	mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
		return llm.NewResponse(
			llm.NewMessage(llm.RoleAssistant, "No API key configured. Run 'harness key setup' to set your OpenAI API key."),
			llm.FinishReasonStop,
		), nil
	})
	return mock
}

func printUsage() {
	fmt.Println(`CageHarness — AI4SE Coding Agent Harness

Usage:
  harness run     <task>       Run a task synchronously
  harness submit  <task>       Submit a task asynchronously
  harness status  <task-id>    Check task status
  harness list                 List all tasks
  harness cancel  <task-id>    Cancel a running task
  harness serve   [port]       Start WebUI server (default :8080)
  harness key     <command>    Manage API keys (setup/status/clear)`)
}
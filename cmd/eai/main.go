package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"cli-agent/internal/app"
	"cli-agent/internal/tui"

	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

const monkeyBanner = `  .-"".-.
 / .===. \
 \/ 6 6 \/
 ( \___/ )
___ooo__V__ooo___`

// Agent command variables
var (
	agentMaxLoops  int
	agentTask      string
	agentMock      bool
)

func main() {
	root := &cobra.Command{
		Use:   "eai",
		Short: "Interactive CLI chat with TUI",
		Long: `Interactive CLI chat with MiniMax API.

Use without arguments for TUI mode, or with the 'agent' subcommand for automated task execution.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := app.DefaultConfigPath()
			cfg, err := app.LoadConfig(configPath)
			if err != nil {
				return err
			}
			if cfg.MinimaxAPIKey == "" {
				cfg.MinimaxAPIKey = os.Getenv("MINIMAX_API_KEY")
			}
			if cfg.BaseURL == "" {
				cfg.BaseURL = os.Getenv("MINIMAX_BASE_URL")
			}
			if cfg.MinimaxAPIKey == "" {
				return fmt.Errorf("MINIMAX_API_KEY not set. Please set it in config or environment")
			}
			application, err := app.NewApplication(cfg, false)
			if err != nil {
				return err
			}
			modeFlag, _ := cmd.Flags().GetString("mode")
			mode, ok := app.ParseMode(modeFlag)
			if !ok {
				mode, _ = app.ParseMode(cfg.DefaultMode)
			}

			// Run TUI
			p := tea.NewProgram(tui.New(application, mode))
			if _, err := p.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	root.Flags().String("mode", "plan", "mode: ask|architect|plan|do|code|debug|orchestrate")
	root.Flags().BoolP("no-tui", "n", false, "Use simple REPL instead of TUI")

	// Add agent subcommand
	agentCmd := &cobra.Command{
		Use:   "agent [task]",
		Short: "Run the CLI agent with MiniMax API",
		Long: `Run an iterative CLI agent that uses MiniMax API to accomplish tasks.

Examples:
  # Interactive mode (reads from stdin)
  eai agent
  
  # Single task
  eai agent "List Go files in the current directory"
  
  # With custom max iterations
  eai agent --max-loops 20 "Analyze the project structure"
  
  # Mock mode (testing without API key)
  eai agent --mock "List files"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle interrupts
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				cancel()
			}()

			// Load configuration
			configPath := app.DefaultConfigPath()
			cfg, err := app.LoadConfig(configPath)
			if err != nil {
				return err
			}

			// Check for mock mode
			if agentMock {
				cfg.MinimaxAPIKey = "mock"
				cfg.Model = "mock"
				fmt.Printf("🔧 Using MOCK mode (no MiniMax API required)\n")
			} else {
				// Check for API key
				apiKey := os.Getenv("MINIMAX_API_KEY")
				if apiKey == "" && cfg.MinimaxAPIKey == "" {
					return fmt.Errorf("MINIMAX_API_KEY not set. Please set it in config or environment")
				}
				if apiKey != "" {
					cfg.MinimaxAPIKey = apiKey
				}
			}

			// Create application
			appInstance, err := app.NewApplication(cfg, agentMock)
			if err != nil {
				return err
			}

			// Determine task
			task := ""
			if len(args) > 0 {
				task = args[0]
			} else if agentTask != "" {
				task = agentTask
			} else {
				// Interactive mode - read from stdin
				fmt.Println("Enter your task (Ctrl+D when done):")
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("error reading input: %w", err)
				}
				task = strings.TrimSpace(string(data))
			}

			if task == "" {
				return fmt.Errorf("no task provided")
			}

			// Create agent loop
			stateDir := filepath.Join(os.TempDir(), "cli-agent", "states")
			agent := app.NewAgentLoop(appInstance.Client, agentMaxLoops, stateDir, appInstance.Logger)

			// Print header
			fmt.Printf("\n🤖 CLI Agent\n")
			fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			fmt.Printf("Task: %s\n", task)
			fmt.Printf("Max iterations: %d\n", agentMaxLoops)
			fmt.Printf("Model: %s\n", cfg.Model)
			fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

			// Execute agent
			start := time.Now()
			state, err := agent.Execute(ctx, task)
			duration := time.Since(start)

			// Print results
			fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			fmt.Printf("✅ Agent Execution Complete\n")
			fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			fmt.Printf("Duration: %v\n", duration)
			fmt.Printf("Iterations: %d\n", state.Iteration)
			fmt.Printf("Tools executed: %d\n", len(state.Results))
			fmt.Printf("Completed: %v\n\n", state.Completed)

			if state.FinalOutput != "" {
				fmt.Printf("Final Output:\n%s\n", state.FinalOutput)
			}

			// Print tool results summary
			if len(state.Results) > 0 {
				fmt.Printf("\nTool Results:\n")
				fmt.Printf("──────────────────────────────────────────────────────\n")
				for i, result := range state.Results {
					status := "✅"
					if !result.Success {
						status = "❌"
					}
					fmt.Printf("%d. %s %s (%dms)\n", i+1, status, result.ToolCallID, result.DurationMs)
					if result.Error != "" {
						fmt.Printf("   Error: %s\n", result.Error)
					}
				}
			}

			fmt.Printf("\n")
			return nil
		},
	}

	agentCmd.Flags().IntVarP(&agentMaxLoops, "max-loops", "l", 10, "Maximum number of agent iterations")
	agentCmd.Flags().StringVarP(&agentTask, "task", "t", "", "Task to execute (non-interactive)")
	agentCmd.Flags().BoolVarP(&agentMock, "mock", "m", false, "Use mock MiniMax client for testing (no API key required)")
	agentCmd.Flags().String("mode", "plan", "mode: ask|architect|plan|do|code|debug|orchestrate")

	root.AddCommand(agentCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

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

const (
	version = "1.0.0"
	repoURL = "https://github.com/clawdbot/clawd"
)

func getBinaryPath() string {
	exe, _ := os.Executable()
	return exe
}

func generateCompletion(shell string) error {
	switch shell {
	case "bash":
		fmt.Println("# bash completion for eai")
		fmt.Println("_eai_completions() {")
		fmt.Println("    local cur prev opts")
		fmt.Println("    COMPREPLY=()")
		fmt.Println("    cur=\"${COMP_WORDS[COMP_CWORD]}\"")
		fmt.Println("    prev=\"${COMP_WORDS[COMP_CWORD-1]}\"")
		fmt.Println("    opts=\"agent completion help version ask architect plan do code debug orchestrate --mock --max-loops --mode --no-tui --help\"")
		fmt.Println("    if [[ $COMP_CWORD -eq 1 ]]; then")
		fmt.Println("        COMPREPLY=( $(compgen -W \"${opts}\" -- \"${cur}\" )")
		fmt.Println("    fi")
		fmt.Println("    return 0")
		fmt.Println("}")
		fmt.Println("complete -F _eai_completions eai")
	case "zsh":
		fmt.Println("# zsh completion for eai")
		fmt.Println("compdef _eai eai")
		fmt.Println("_eai() {")
		fmt.Println("    _arguments -C \\")
		fmt.Println("        '(-h --help)'{-h,--help}'[show help]' \\")
		fmt.Println("        '(-v --version)'{-v,--version}'[print version]' \\")
		fmt.Println("        '(-n --no-tui)'{-n,--no-tui}'[use simple REPL instead of TUI]' \\")
		fmt.Println("        '(-m --mode)'{-m,--mode}'[set mode]:mode:(ask architect plan do code debug orchestrate)' \\")
		fmt.Println("        '*::command:->command'")
		fmt.Println("    case $state in")
		fmt.Println("        command)")
		fmt.Println("            if (( CURRENT == 1 )); then")
		fmt.Println("                _describe -t commands 'eai commands' commands")
		fmt.Println("            fi")
		fmt.Println("            ;;")
		fmt.Println("    esac")
		fmt.Println("}")
	case "fish":
		fmt.Println("# fish completion for eai")
		fmt.Println("complete -c eai -f -a '(agent completion help version ask architect plan do code debug orchestrate)'")
		fmt.Println("complete -c eai -s h -l help -d 'Show help'")
		fmt.Println("complete -c eai -s v -l version -d 'Print version'")
		fmt.Println("complete -c eai -s n -l no-tui -d 'Use simple REPL'")
		fmt.Println("complete -c eai -s m -l mode -d 'Set mode' -a 'ask architect plan do code debug orchestrate'")
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
	return nil
}

func main() {
	root := &cobra.Command{
		Use:   "eai",
		Short: "EAI - CLI Agent with MiniMax API",
		Long:  "EAI is an interactive CLI agent powered by MiniMax API.\n\nUse without arguments for TUI mode, or with the 'agent' subcommand for automated task execution.\n\nFor more information, visit: " + repoURL,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			if v, _ := cmd.Flags().GetBool("version"); v {
				fmt.Printf("EAI CLI Agent v%s\n", version)
				fmt.Printf("Repository: %s\n", repoURL)
				fmt.Printf("Installed at: %s\n", getBinaryPath())
				return nil
			}
			
			if comp, _ := cmd.Flags().GetString("completion"); comp != "" {
				return generateCompletion(comp)
			}

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
			
			mockMode := cfg.MinimaxAPIKey == ""
			application, err := app.NewApplication(cfg, mockMode)
			if err != nil {
				return err
			}
			
			modeFlag, _ := cmd.Flags().GetString("mode")
			mode, ok := app.ParseMode(modeFlag)
			if !ok {
				mode, _ = app.ParseMode(cfg.DefaultMode)
			}

			p := tea.NewProgram(tui.New(application, mode, mockMode))
			if _, err := p.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	root.Flags().String("mode", "plan", "mode: ask|architect|plan|do|code|debug|orchestrate")
	root.Flags().BoolP("no-tui", "n", false, "Use simple REPL instead of TUI")
	root.Flags().BoolP("version", "v", false, "Print version information")
	root.Flags().String("completion", "", "Generate shell completion (bash|zsh|fish)")

	agentCmd := &cobra.Command{
		Use:   "agent [task]",
		Short: "Run the CLI agent with MiniMax API",
		Long:  "Run an iterative CLI agent that uses MiniMax API to accomplish tasks.\n\nExamples:\n  - eai agent\n  - eai agent \"List Go files\"\n  - eai agent --max-loops 20 \"Analyze\"\n  - eai agent --mock \"List files\"",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				cancel()
			}()

			configPath := app.DefaultConfigPath()
			cfg, err := app.LoadConfig(configPath)
			if err != nil {
				return err
			}
			
			if agentMock {
				cfg.MinimaxAPIKey = "mock"
				cfg.Model = "mock"
			} else {
				if cfg.MinimaxAPIKey == "" {
					cfg.MinimaxAPIKey = os.Getenv("MINIMAX_API_KEY")
				}
				if cfg.BaseURL == "" {
					cfg.BaseURL = os.Getenv("MINIMAX_BASE_URL")
				}
				if cfg.MinimaxAPIKey == "" {
					return fmt.Errorf("MINIMAX_API_KEY not set. Please set it in config or environment")
				}
			}

			appInstance, err := app.NewApplication(cfg, agentMock)
			if err != nil {
				return err
			}

			task := ""
			if len(args) > 0 {
				task = args[0]
			} else if agentTask != "" {
				task = agentTask
			} else {
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

			stateDir := filepath.Join(os.TempDir(), "cli-agent", "states")
			agent := app.NewAgentLoop(appInstance.Client, agentMaxLoops, stateDir, appInstance.Logger)

			start := time.Now()
			state, err := agent.Execute(ctx, task)
			duration := time.Since(start)

			fmt.Printf("\nAgent Execution Complete\n")
			fmt.Printf("Duration: %v\n", duration)
			fmt.Printf("Iterations: %d\n", state.Iteration)
			fmt.Printf("Tools executed: %d\n", len(state.Results))
			fmt.Printf("Completed: %v\n\n", state.Completed)

			if state.FinalOutput != "" {
				fmt.Printf("Final Output:\n%s\n", state.FinalOutput)
			}

			if len(state.Results) > 0 {
				fmt.Printf("\nTool Results:\n")
				for i, result := range state.Results {
					status := "SUCCESS"
					if !result.Success {
						status = "FAILURE"
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
	agentCmd.Flags().BoolVarP(&agentMock, "mock", "m", false, "Use mock MiniMax client for testing")

	root.AddCommand(agentCmd)

	completionCmd := &cobra.Command{
		Use:   "completion [shell]",
		Short: "Generate shell completion",
		Long:  "Generate shell completion script for eai.\n\nExamples:\n  - eai completion bash >> ~/.bashrc\n  - eai completion zsh > ~/.zsh/completion/_eai\n  - eai completion fish > ~/.config/fish/completions/eai.fish",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCompletion(args[0])
		},
	}
	root.AddCommand(completionCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var (
	agentMaxLoops  int
	agentTask      string
	agentMock      bool
)

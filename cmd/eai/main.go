package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
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
	repoURL = "https://github.com/jaivial/cli-agent"
)

const permissionsValues = "full-access|dangerously-full-access"

func applyEnvOverrides(cfg *app.Config) {
	if v := strings.TrimSpace(os.Getenv("EAI_API_KEY")); v != "" {
		cfg.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("EAI_BASE_URL")); v != "" {
		cfg.BaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("EAI_MODEL")); v != "" {
		cfg.Model = v
	}
	if v := strings.TrimSpace(os.Getenv("EAI_MAX_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxTokens = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("EAI_PERMISSIONS")); v != "" {
		cfg.Permissions = app.NormalizePermissionsMode(v)
	}
	cfg.Model = app.NormalizeModel(cfg.Model)
	cfg.BaseURL = app.NormalizeBaseURL(cfg.BaseURL)
	cfg.Permissions = app.NormalizePermissionsMode(cfg.Permissions)
}

func applyFlagOverrides(cmd *cobra.Command, cfg *app.Config) error {
	if cfg == nil || cmd == nil {
		return nil
	}
	if v, _ := cmd.Flags().GetString("permissions"); strings.TrimSpace(v) != "" {
		mode, ok := app.ParsePermissionsMode(v)
		if !ok {
			return fmt.Errorf("invalid --permissions value %q (use %s)", v, permissionsValues)
		}
		cfg.Permissions = mode
	}
	return nil
}

func formatPermissionsStatus(desired string) string {
	desiredMode := app.NormalizePermissionsMode(desired)
	effectiveMode, isRoot := app.EffectivePermissionsMode(desiredMode)

	var b strings.Builder
	b.WriteString("permissions status:\n")
	b.WriteString("- desired: ")
	b.WriteString(desiredMode)
	b.WriteString("\n")
	b.WriteString("- effective: ")
	b.WriteString(effectiveMode)
	b.WriteString("\n")
	if isRoot {
		b.WriteString("- running as root: yes\n")
	} else {
		b.WriteString("- running as root: no\n")
	}
	if desiredMode == app.PermissionsDangerouslyFullAccess && !isRoot {
		b.WriteString("- note: dangerously-full-access requires launching eai as root (example: sudo -E eai)\n")
	}
	return strings.TrimSpace(b.String())
}

func handlePermissionsSlash(input string, cfg *app.Config, application *app.Application) (bool, string) {
	trimmed := strings.TrimSpace(input)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "/permissions") {
		return false, ""
	}

	arg := strings.TrimSpace(trimmed[len("/permissions"):])
	if arg == "" {
		return true, formatPermissionsStatus(cfg.Permissions)
	}

	mode, ok := app.ParsePermissionsMode(arg)
	if !ok {
		return true, fmt.Sprintf("invalid permissions mode %q. use: /permissions %s", arg, permissionsValues)
	}

	cfg.Permissions = mode
	if application != nil {
		application.Config.Permissions = mode
	}
	if err := app.SaveConfig(*cfg, app.DefaultConfigPath()); err != nil {
		return true, fmt.Sprintf("permissions updated in-memory (%s), but failed to save config: %v\n\n%s", mode, err, formatPermissionsStatus(mode))
	}
	return true, fmt.Sprintf("permissions updated to %s\n\n%s", mode, formatPermissionsStatus(mode))
}

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
		fmt.Println("    opts=\"agent install resume completion help version ask architect plan do code debug orchestrate --mock --max-loops --mode --permissions --no-tui --help\"")
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
		fmt.Println("        '--permissions[set permissions mode]:permissions:(full-access dangerously-full-access)' \\")
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
		fmt.Println("complete -c eai -f -a '(agent install resume completion help version ask architect plan do code debug orchestrate)'")
		fmt.Println("complete -c eai -s h -l help -d 'Show help'")
		fmt.Println("complete -c eai -s v -l version -d 'Print version'")
		fmt.Println("complete -c eai -s n -l no-tui -d 'Use simple REPL'")
		fmt.Println("complete -c eai -s m -l mode -d 'Set mode' -a 'ask architect plan do code debug orchestrate'")
		fmt.Println("complete -c eai -l permissions -d 'Set permissions mode' -a 'full-access dangerously-full-access'")
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
	return nil
}

func main() {
	root := &cobra.Command{
		Use:     "eai",
		Short:   "EAI - CLI agent powered by Z.AI",
		Long:    "EAI is an interactive CLI agent powered by Z.AI models.\n\nUse without arguments for TUI mode, or with the 'agent' subcommand for automated task execution.\n\nFor more information, visit: " + repoURL,
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
			applyEnvOverrides(&cfg)
			if err := applyFlagOverrides(cmd, &cfg); err != nil {
				return err
			}

			mockMode, _ := cmd.Flags().GetBool("mock")
			application, err := app.NewApplication(cfg, mockMode)
			if err != nil {
				return err
			}

			modeFlag, _ := cmd.Flags().GetString("mode")
			mode, ok := app.ParseMode(modeFlag)
			if !ok {
				mode, _ = app.ParseMode(cfg.DefaultMode)
			}

			noTUI, _ := cmd.Flags().GetBool("no-tui")
			if noTUI {
				// Very small fallback REPL for environments that can't render a TUI.
				fmt.Println("eai (no-tui). type your message and press enter. ctrl+d to exit.")
				in := bufio.NewScanner(os.Stdin)
				for {
					fmt.Print("> ")
					if !in.Scan() {
						return nil
					}
					line := strings.TrimSpace(in.Text())
					if line == "" {
						continue
					}
					if handled, msg := handlePermissionsSlash(line, &cfg, application); handled {
						fmt.Println(msg)
						continue
					}
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
					out, err := application.ExecuteChat(ctx, mode, line)
					cancel()
					if err != nil {
						fmt.Fprintf(os.Stderr, "error: %v\n", err)
						continue
					}
					fmt.Println(out)
				}
			}

			var opts []tea.ProgramOption
			if cfg.UseAltScreen {
				opts = append(opts, tea.WithAltScreen())
			}
			if cfg.EnableMouse {
				opts = append(opts, tea.WithMouseCellMotion())
			}
			p := tea.NewProgram(tui.NewMainModel(application, mode), opts...)
			_, err = p.Run()
			return err
		},
	}

	root.Flags().String("mode", "plan", "mode: plan|create (TUI), plus ask|architect|do|code|debug|orchestrate")
	root.Flags().BoolP("no-tui", "n", false, "Use simple REPL instead of TUI")
	root.Flags().Bool("mock", false, "Use mock client (no API calls)")
	root.Flags().BoolP("version", "v", false, "Print version information")
	root.Flags().String("completion", "", "Generate shell completion (bash|zsh|fish)")
	root.PersistentFlags().String("permissions", "", "permissions mode: full-access|dangerously-full-access")

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install eai onto your PATH (default: ~/.local/bin/eai)",
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := os.Executable()
			if err != nil {
				return err
			}
			dest, _ := cmd.Flags().GetString("dest")
			if dest == "" {
				home, _ := os.UserHomeDir()
				dest = filepath.Join(home, ".local", "bin", "eai")
			}
			if st, err := os.Stat(dest); err == nil && st.IsDir() {
				dest = filepath.Join(dest, "eai")
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}

			in, err := os.Open(src)
			if err != nil {
				return err
			}
			defer in.Close()

			tmp := dest + ".tmp"
			out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
			if err != nil {
				return fmt.Errorf("failed to write %s: %w", dest, err)
			}
			defer out.Close()

			if _, err := io.Copy(out, in); err != nil {
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			if err := os.Rename(tmp, dest); err != nil {
				return err
			}

			fmt.Printf("installed %s\n", dest)
			return nil
		},
	}
	installCmd.Flags().String("dest", "", "Install destination file or directory (default: ~/.local/bin/eai)")
	root.AddCommand(installCmd)

	agentCmd := &cobra.Command{
		Use:   "agent [task]",
		Short: "Run the CLI agent with Z.AI",
		Long:  "Run an iterative CLI agent that uses Z.AI to accomplish tasks.\n\nExamples:\n  - eai agent\n  - eai agent \"List Go files\"\n  - eai agent --max-loops 20 \"Analyze\"\n  - eai agent --mock \"List files\"",
		Args:  cobra.MaximumNArgs(1),
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
				cfg.APIKey = "mock"
				cfg.Model = "mock"
			} else {
				applyEnvOverrides(&cfg)
				if cfg.APIKey == "" {
					return fmt.Errorf("EAI_API_KEY not set. Please set it in config or environment")
				}
			}
			if err := applyFlagOverrides(cmd, &cfg); err != nil {
				return err
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

	agentCmd.Flags().IntVarP(&agentMaxLoops, "max-loops", "l", 30, "Maximum number of agent iterations")
	agentCmd.Flags().StringVarP(&agentTask, "task", "t", "", "Task to execute (non-interactive)")
	agentCmd.Flags().BoolVarP(&agentMock, "mock", "m", false, "Use mock Z.AI client for testing")

	root.AddCommand(agentCmd)

	completionCmd := &cobra.Command{
		Use:   "completion [shell]",
		Short: "Generate shell completion",
		Long:  "Generate shell completion script for eai.\n\nExamples:\n  - eai completion bash >> ~/.bashrc\n  - eai completion zsh > ~/.zsh/completion/_eai\n  - eai completion fish > ~/.config/fish/completions/eai.fish",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCompletion(args[0])
		},
	}
	root.AddCommand(completionCmd)

	resumeCmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume a previous chat session in this folder",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := app.DefaultConfigPath()
			cfg, err := app.LoadConfig(configPath)
			if err != nil {
				return err
			}
			applyEnvOverrides(&cfg)
			if err := applyFlagOverrides(cmd, &cfg); err != nil {
				return err
			}

			mockMode, _ := cmd.Flags().GetBool("mock")
			application, err := app.NewApplication(cfg, mockMode)
			if err != nil {
				return err
			}

			modeFlag, _ := cmd.Flags().GetString("mode")
			mode, ok := app.ParseMode(modeFlag)
			if !ok {
				mode, _ = app.ParseMode(cfg.DefaultMode)
			}

			var opts []tea.ProgramOption
			if cfg.UseAltScreen {
				opts = append(opts, tea.WithAltScreen())
			}
			if cfg.EnableMouse {
				opts = append(opts, tea.WithMouseCellMotion())
			}

			model := tui.NewMainModel(application, mode)
			model.StartResumePicker()

			p := tea.NewProgram(model, opts...)
			_, err = p.Run()
			return err
		},
	}
	resumeCmd.Flags().String("mode", "create", "mode: plan|create")
	resumeCmd.Flags().Bool("mock", false, "Use mock client (no API calls)")
	root.AddCommand(resumeCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var (
	agentMaxLoops int
	agentTask     string
	agentMock     bool
)

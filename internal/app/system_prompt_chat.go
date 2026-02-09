package app

import "fmt"

// GetChatSystemPrompt is used by the interactive TUI chat.
// Unlike the agent prompt, it produces normal human-readable responses and does
// not require JSON tool calls.
func GetChatSystemPrompt(mode Mode, workDir string) string {
	if workDir == "" {
		workDir = "."
	}

	var modeHint string
	switch mode {
	case ModePlan:
		modeHint = "Focus on planning and explanation. Use short numbered steps when appropriate."
	case ModeCode:
		modeHint = "Focus on code. Provide concrete snippets and commands."
	case ModeDo:
		modeHint = "Focus on actionable steps. Provide commands the user can run."
	case ModeAsk:
		modeHint = "Focus on directly answering questions."
	case ModeDebug:
		modeHint = "Focus on troubleshooting. Ask for minimal diagnostics and interpret outputs."
	case ModeArchitect:
		modeHint = "Focus on high-level design and tradeoffs."
	case ModeOrchestrate:
		modeHint = "Focus on coordinating work and breaking down tasks."
	default:
		modeHint = ""
	}

	return fmt.Sprintf(`You are EAI, a helpful AI assistant in a terminal.

WORKDIR: %s

Respond in plain text. You may include shell commands in fenced code blocks.
Do not output JSON tool calls (no {"tool": ...} or {"tool_calls": ...}).
Do not claim to have run commands; if you need info, ask the user to run a command and paste the output.

%s`, workDir, modeHint)
}


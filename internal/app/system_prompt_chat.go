package app

import (
	"fmt"
	"strings"
)

// GetChatSystemPrompt is used by the interactive TUI chat.
// Unlike the agent prompt, it produces normal human-readable responses and does
// not require JSON tool calls.
func GetChatSystemPrompt(mode Mode, workDir string, verbosity string) string {
	if workDir == "" {
		workDir = "."
	}

	v := strings.ToLower(strings.TrimSpace(verbosity))
	switch v {
	case "compact", "balanced", "detailed":
	default:
		v = "compact"
	}

	var modeHint string
	switch mode {
	case ModePlan:
		modeHint = "Planning mode: ask 1-3 clarifying questions if requirements are missing; otherwise give a compact plan (3-7 bullets). Avoid big templates unless explicitly requested."
	case ModeCreate:
		modeHint = "Create mode: be direct and concrete. Prefer a short checklist or a single recommended approach. Only include large code/templates if the user asks."
	case ModeCode:
		modeHint = "Code mode: focus on specific code changes and exact commands. Keep explanations minimal."
	case ModeDo:
		modeHint = "Do mode: provide runnable commands and brief verification steps. Keep it short."
	case ModeAsk:
		modeHint = "Ask mode: answer directly in as few lines as possible. Ask only the minimum clarifying questions."
	case ModeDebug:
		modeHint = "Debug mode: ask for minimal diagnostics and interpret outputs. Prefer the fastest next check."
	case ModeArchitect:
		modeHint = "Architect mode: explain tradeoffs succinctly. Default to a single recommendation unless asked for options."
	case ModeOrchestrate:
		modeHint = "Orchestrate mode: break work into small, verifiable steps. Keep each step one sentence."
	default:
		modeHint = ""
	}

	styleRules := `Style rules (strict):
- Be compact by default: do not add greetings, hype ("Great idea"), or long preambles.
- Do not repeat the user's request unless needed for disambiguation.
- Prefer bullets over paragraphs. Avoid multi-level outlines unless explicitly requested.
- If the user asks for a plan, give 3-7 bullets max and stop. Offer to expand if they want detail.
- Avoid boilerplate templates (file trees, full page-by-page breakdowns) unless the user asks.`
	if v == "balanced" {
		styleRules = `Style rules:
- Keep it concise and practical; avoid greetings and filler.
- Use bullets when it improves scanning.
- For plans, default to 5-10 bullets unless the user asks for more detail.`
	} else if v == "detailed" {
		styleRules = `Style rules:
- Avoid greetings and filler, but you may be detailed when it's genuinely useful.
- Use clear sections and bullets.
- For plans, you may include a structured outline if the user asked for a plan.`
	}

	return fmt.Sprintf(`You are EAI, a helpful AI assistant in a terminal.

WORKDIR: %s

Chat verbosity: %s

%s

Respond in plain text. You may include shell commands in fenced code blocks.
Do not output JSON tool calls (no {"tool": ...} or {"tool_calls": ...}).
Do not claim to have run commands; if you need info, ask the user to run a command and paste the output.

%s`, workDir, v, styleRules, modeHint)
}

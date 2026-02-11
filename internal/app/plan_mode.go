package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	proposedPlanBlockRE = regexp.MustCompile(`(?is)<proposed_plan>\s*(.*?)\s*</proposed_plan>`)
	planNumberedLineRE  = regexp.MustCompile(`^\d+[.)]\s+`)
)

func isMockClient(client *MinimaxClient) bool {
	if client == nil {
		return false
	}
	return client.APIKey == "mock" || client.BaseURL == "mock://"
}

// PlanDiscoveryTools are read-only tools used in plan mode to gather context
// before producing an implementation plan.
func PlanDiscoveryTools() []Tool {
	allowed := map[string]bool{
		"list_dir":     true,
		"search_files": true,
		"grep":         true,
		"read_file":    true,
	}
	var tools []Tool
	for _, tool := range DefaultTools() {
		if allowed[tool.Name] {
			tools = append(tools, tool)
		}
	}
	return tools
}

func GetPlanAgentSystemPrompt(workDir string) string {
	if strings.TrimSpace(workDir) == "" {
		workDir = "."
	}

	return fmt.Sprintf(`You are EAI in PLAN mode. Your job is to inspect the project and return an implementation plan.

WORKDIR: %s

CRITICAL MODE RULES:
- This is planning-only mode. Do NOT modify files and do NOT run mutating commands.
- You may gather context using read-only tools (list_dir, search_files, grep, read_file).
- Do not ask the user to run shell commands for basic discovery; use tools directly.
- Stop gathering once you have enough context for a concrete plan.

RESPONSE RULES (STRICT):
- Every response must be exactly ONE of:
  1) A single JSON tool call: {"tool":"...", "args":{...}}
  2) A final plan wrapped in <proposed_plan> ... </proposed_plan>
- For the final plan:
  - Use 4-10 checklist items.
  - Format each item as: - [ ] ...
  - Keep steps concrete and verifiable.
  - Include at least one explicit verification step.
- No prose before or after the final <proposed_plan> block.`, workDir)
}

func mockPlanResponse(input string) string {
	task := strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
	if task == "" {
		task = "the requested task"
	}
	if len(task) > 120 {
		task = task[:120] + "..."
	}

	return fmt.Sprintf(`<proposed_plan>
- [ ] Confirm scope, constraints, and acceptance criteria for: %s
- [ ] Inspect project structure and identify files/components involved.
- [ ] Compare current behavior against expected outcomes and note gaps.
- [ ] Define ordered implementation steps with minimal, testable changes.
- [ ] Add verification checks for each critical requirement before completion.
</proposed_plan>`, task)
}

func extractPlanChecklistItems(raw string) []string {
	var items []string
	seen := make(map[string]bool)

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if lower == "<proposed_plan>" || lower == "</proposed_plan>" {
			continue
		}

		item := ""
		switch {
		case strings.HasPrefix(lower, "- [ ] "):
			item = strings.TrimSpace(line[6:])
		case strings.HasPrefix(lower, "- [x] "):
			item = strings.TrimSpace(line[6:])
		case strings.HasPrefix(lower, "- "):
			item = strings.TrimSpace(line[2:])
		case strings.HasPrefix(lower, "* "):
			item = strings.TrimSpace(line[2:])
		case planNumberedLineRE.MatchString(line):
			item = strings.TrimSpace(planNumberedLineRE.ReplaceAllString(line, ""))
		}

		item = strings.TrimSpace(item)
		item = strings.Trim(item, "-*")
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
	}

	return items
}

func hasVerificationStep(items []string) bool {
	for _, item := range items {
		l := strings.ToLower(item)
		if strings.Contains(l, "verify") || strings.Contains(l, "test") || strings.Contains(l, "validate") || strings.Contains(l, "check") {
			return true
		}
	}
	return false
}

func isPlanResponseShape(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if m := proposedPlanBlockRE.FindStringSubmatch(raw); len(m) >= 2 {
		return len(extractPlanChecklistItems(m[1])) >= 2
	}
	return len(extractPlanChecklistItems(raw)) >= 3
}

func fallbackPlanResponse(input string) string {
	task := strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
	if task == "" {
		task = "the requested task"
	}
	if len(task) > 140 {
		task = task[:140] + "..."
	}
	return fmt.Sprintf(`<proposed_plan>
- [ ] Clarify scope, constraints, and expected output for: %s
- [ ] Inspect the relevant files and current implementation details.
- [ ] Break implementation into small, ordered, verifiable changes.
- [ ] Define validation checks and acceptance criteria for each change.
- [ ] Execute in Create mode only after plan approval.
</proposed_plan>`, task)
}

func normalizePlanResponse(raw string, input string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallbackPlanResponse(input)
	}

	body := raw
	if m := proposedPlanBlockRE.FindStringSubmatch(raw); len(m) >= 2 {
		body = strings.TrimSpace(m[1])
	}

	items := extractPlanChecklistItems(body)
	if len(items) == 0 {
		return fallbackPlanResponse(input)
	}
	if !hasVerificationStep(items) {
		items = append(items, "Verify assumptions and expected outcomes with quick checks before implementation.")
	}

	var b strings.Builder
	b.WriteString("<proposed_plan>\n")
	for _, item := range items {
		b.WriteString("- [ ] ")
		b.WriteString(strings.TrimSpace(item))
		b.WriteString("\n")
	}
	b.WriteString("</proposed_plan>")
	return b.String()
}

func renderPlanStateForChat(state *AgentState, input string) string {
	if state == nil {
		return fallbackPlanResponse(input)
	}

	candidates := []string{}
	if strings.TrimSpace(state.FinalOutput) != "" {
		candidates = append(candidates, strings.TrimSpace(state.FinalOutput))
	}
	for i := len(state.Messages) - 1; i >= 0; i-- {
		msg := state.Messages[i]
		if msg.Role != "assistant" {
			continue
		}
		txt := strings.TrimSpace(msg.Content)
		if txt != "" {
			candidates = append(candidates, txt)
		}
		if len(candidates) >= 3 {
			break
		}
	}

	for _, cand := range candidates {
		if isPlanResponseShape(cand) {
			return normalizePlanResponse(cand, input)
		}
	}
	for _, cand := range candidates {
		if strings.TrimSpace(cand) != "" {
			return normalizePlanResponse(cand, input)
		}
	}
	return fallbackPlanResponse(input)
}

func (a *Application) executePlanModeWithProgressEvents(
	ctx context.Context,
	input string,
	progress func(ProgressEvent),
) (string, error) {
	if isMockClient(a.Client) {
		if progress != nil {
			now := time.Now()
			progress(ProgressEvent{Kind: "thinking", Text: "Planning approach", At: now})
			progress(ProgressEvent{Kind: "reasoning", Text: "Gathering quick repository context before drafting the checklist.", At: now})
			progress(ProgressEvent{
				Kind:       "tool",
				Text:       "Exploring directories",
				Tool:       "list_dir",
				ToolCallID: "plan_list_dir_mock_1",
				ToolStatus: "pending",
				At:         now,
			})
			progress(ProgressEvent{
				Kind:       "tool",
				Text:       "Directory listing finished",
				Tool:       "list_dir",
				ToolCallID: "plan_list_dir_mock_1",
				ToolStatus: "completed",
				DurationMs: 2,
				At:         now.Add(2 * time.Millisecond),
			})
		}
		return mockPlanResponse(input), nil
	}

	if a.Client != nil && a.Client.APIKey == "" && a.Client.BaseURL != "mock://" {
		if progress != nil {
			progress(ProgressEvent{
				Kind: "reasoning",
				Text: "No API key configured; returning a heuristic plan without repository exploration.",
				At:   time.Now(),
			})
		}
		return fallbackPlanResponse(input), nil
	}

	stateDir := filepath.Join(os.TempDir(), "cli-agent", "states")
	agent := NewAgentLoop(a.Client, 10, stateDir, a.Logger)
	if wd, err := os.Getwd(); err == nil && wd != "" {
		agent.WorkDir = wd
	}
	agent.Progress = progress
	agent.Tools = PlanDiscoveryTools()
	agent.SystemMessageBuilder = func(task string) string {
		return GetPlanAgentSystemPrompt(agent.WorkDir)
	}
	agent.FinalResponseValidator = isPlanResponseShape
	agent.FinalResponseGuidance = "Continue gathering context with read-only tools as needed. When ready, respond ONLY with a <proposed_plan>...</proposed_plan> checklist (4-10 items)."

	toolCtx, cancel := context.WithTimeout(ctx, 9*time.Minute)
	defer cancel()

	state, err := agent.Execute(toolCtx, input)
	if err != nil {
		if progress != nil {
			progress(ProgressEvent{
				Kind: "reasoning",
				Text: "Planning run failed; returning a heuristic fallback plan.",
				At:   time.Now(),
			})
		}
		return fallbackPlanResponse(input), nil
	}
	return renderPlanStateForChat(state, input), nil
}

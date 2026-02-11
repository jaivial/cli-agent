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
	planCheckboxLineRE  = regexp.MustCompile(`(?i)^\s*(?:[-*+]\s*)?\[\s*(?: |x)\s*\]\s+(.+)$`)
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

MODE RULES (STRICT):
- You are in Plan mode until the app changes modes. If the user asks to implement/execute, treat it as a request to plan the execution, not do it.
- Plan mode is planning-only. Do NOT modify files and do NOT run mutating commands.
- Ground the plan in the actual repo: use read-only tools (list_dir, search_files, grep, read_file) to discover facts before finalizing.
- Prefer discovering "unknowns" from the repo over asking the user. Only ask when it is truly a preference/tradeoff not inferable from the codebase.
- Stop exploring once you have enough context for a decision-complete plan.
- WORKDIR is a default starting point, not a boundary. If the user request clearly points to files outside WORKDIR, inspect those exact paths.
- Path tips:
  - You can use `+"`~`"+` (home) in tool paths (e.g., `+"`~/Desktop/eai`"+`).
  - If the user references common home folders (Desktop/Downloads/Documents/etc), prefer checking under home first (e.g., `+"`Desktop/eai`"+` or `+"`~/Desktop/eai`"+`).

TWO KINDS OF UNKNOWNS (TREAT DIFFERENTLY):
1) Discoverable facts (repo truth): explore first using tools.
2) Preferences/tradeoffs (not discoverable): choose a sensible default, record it under "Assumptions", and proceed.

FINAL PLAN FORMAT (REQUIRED):
- Return exactly one <proposed_plan>...</proposed_plan> block and nothing else.
- Inside the block, include (in order):
  1) A short title (one line)
  2) A brief Summary section (2-5 bullets)
  3) An Assumptions section (bullets; include defaults you chose)
  4) A Plan section with 6-12 checklist items formatted as: - [ ] ...
  5) A Verification section with 2-5 checklist items formatted as: - [ ] ...
- The plan must be decision-complete: the implementer should not have to make any decisions.

RESPONSE RULES (STRICT):
- Every response must be exactly ONE of:
  1) A single JSON tool call: {"tool":"...", "args":{...}}
  2) A final plan wrapped in <proposed_plan> ... </proposed_plan>
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
# Plan: %s

## Summary
- Produce a decision-complete plan grounded in the repo.
- Minimize assumptions; record any defaults explicitly.

## Assumptions
- The implementation will prioritize minimal, testable changes over refactors.

## Plan
- [ ] Confirm scope, constraints, and acceptance criteria for: %s
- [ ] Inspect project structure and identify the specific files/components involved.
- [ ] Describe the intended behavior precisely (inputs/outputs, edge cases, errors).
- [ ] Outline the minimal code changes required, in order, with concrete file targets.
- [ ] Call out any public API/interface/schema changes (if applicable).
- [ ] Note any risks, compatibility concerns, or migrations (if applicable).

## Verification
- [ ] Run the most relevant unit/integration tests (or add a small smoke check if none exist).
- [ ] Validate the change against the stated acceptance criteria.
</proposed_plan>`, task, task)
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
		case planCheckboxLineRE.MatchString(line):
			m := planCheckboxLineRE.FindStringSubmatch(line)
			if len(m) >= 2 {
				item = strings.TrimSpace(m[1])
			}
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

func wrapProposedPlan(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	return "<proposed_plan>\n" + body + "\n</proposed_plan>"
}

func ensureVerificationInPlanBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return body
	}

	// Prefer checkbox-style items for verification detection, but fall back to list items.
	items := extractPlanChecklistItems(body)
	if hasVerificationStep(items) {
		return body
	}

	// Append a small verification section rather than silently changing existing steps.
	var b strings.Builder
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n## Verification\n")
	b.WriteString("- [ ] Run a quick verification (tests/build/smoke) that exercises the changed behavior.\n")
	return strings.TrimSpace(b.String())
}

func isPlanResponseShape(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}

	body := raw
	if m := proposedPlanBlockRE.FindStringSubmatch(raw); len(m) >= 2 {
		body = strings.TrimSpace(m[1])
		if body == "" {
			return false
		}
	}

	items := extractPlanChecklistItems(body)
	if len(items) >= 4 {
		return true
	}
	// Be permissive if a <proposed_plan> block exists and contains some concrete steps.
	return len(items) >= 2 && proposedPlanBlockRE.MatchString(raw)
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
# Plan: %s

## Summary
- Outline the smallest set of concrete changes to deliver the request.
- Record assumptions and provide verification steps.

## Assumptions
- Missing preferences will use sensible defaults and be called out explicitly.

## Plan
- [ ] Clarify scope, constraints, and expected output for: %s
- [ ] Inspect the relevant files and current implementation details.
- [ ] Break the implementation into small, ordered, verifiable changes.
- [ ] Identify exact files/functions/commands involved for each step.
- [ ] Specify any configuration/env changes and the chosen defaults.
- [ ] Note edge cases, failure modes, and any compatibility concerns.

## Verification
- [ ] Run a targeted build/test/smoke check that covers the core behavior.
- [ ] Confirm acceptance criteria are met.
</proposed_plan>`, task, task)
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

	// If the model returned a bare checklist without tags, wrap it.
	items := extractPlanChecklistItems(body)
	if len(items) == 0 {
		return fallbackPlanResponse(input)
	}

	body = ensureVerificationInPlanBody(body)
	return wrapProposedPlan(body)
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
	agent.FinalResponseGuidance = "Continue gathering context with read-only tools as needed. When ready, respond ONLY with a single <proposed_plan>...</proposed_plan> block containing: title, summary, assumptions, plan checklist, verification checklist."

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

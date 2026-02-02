package app

import (
	"fmt"
	"os"
	"strings"
)

// PromptBuilder constructs system prompts with mode and category-specific guidance.
type PromptBuilder struct {
	categoryHints []string // Optional category hints provided by caller
}

// NewPromptBuilder creates a new prompt builder.
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		categoryHints: []string{},
	}
}

// NewPromptBuilderWithHints creates a PromptBuilder with pre-defined category hints
func NewPromptBuilderWithHints(hints []string) *PromptBuilder {
	return &PromptBuilder{
		categoryHints: hints,
	}
}

// GetProjectContext returns information about the current working directory
func GetProjectContext() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	entries, err := os.ReadDir(cwd)
	if err != nil {
		return fmt.Sprintf("cwd: %s", cwd)
	}

	var dirs, files []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, name+"/")
		} else {
			files = append(files, name)
		}
	}

	result := fmt.Sprintf("cwd: %s\n", cwd)
	if len(dirs) > 0 {
		result += "dirs: " + strings.Join(dirs, " ")
	}
	if len(files) > 0 {
		if len(dirs) > 0 {
			result += "\n"
		}
		result += "files: " + strings.Join(files, " ")
	}

	if len(result) > 500 {
		result = result[:497] + "..."
	}
	return result
}

// SystemPrompt generates a system prompt for the given mode and optional category hints.
func (p *PromptBuilder) SystemPrompt(mode Mode, categoryHints ...string) string {
	// Merge provided hints with any pre-configured hints
	allHints := append(p.categoryHints, categoryHints...)

	// Use the enhanced system prompt as the base
	basePrompt := GetEnhancedSystemPrompt()

	// Add category-specific guidance if hints provided
	var categoryGuidance string
	if len(allHints) > 0 {
		categoryGuidance = p.buildCategoryGuidance(allHints)
	}

	var modePrompt string
	switch mode {
	case ModePlan:
		modePrompt = `

## MODE: PLAN
Analyze project structure, then provide numbered action plan.`

	case ModeCode:
		modePrompt = `

## MODE: CODE
Read files first, then make focused code changes.`

	case ModeDo:
		modePrompt = `

## MODE: ACT
Execute tasks step by step. Report completion when done.`

	case ModeAsk:
		modePrompt = `

## MODE: ASK
Search and read relevant files to answer questions.`

	default:
		modePrompt = ""
	}

	result := basePrompt + modePrompt
	if categoryGuidance != "" {
		result += "\n\n## TASK-SPECIFIC GUIDANCE\n" + categoryGuidance
	}

	return result
}

// SystemPromptWithTask generates a prompt based on automatic task analysis.
func (p *PromptBuilder) SystemPromptWithTask(mode Mode, taskDescription string) string {
	// Detect category from task description
	category := detectCategory(taskDescription)

	// Check for compound tasks
	if detectCompoundTasks(taskDescription) {
		return p.buildCompoundTaskPrompt(mode, taskDescription)
	}

	// Get the base prompt with category hint
	return p.SystemPrompt(mode, category)
}

// buildCategoryGuidance extracts relevant guidance from category prompts
func (p *PromptBuilder) buildCategoryGuidance(categories []string) string {
	var guidance []string

	for _, cat := range categories {
		if prompt, ok := categoryPrompts[cat]; ok && cat != "default" {
			// Extract key sections from the category prompt
			guidance = append(guidance, fmt.Sprintf("### %s\n%s", cat, prompt))
		}
	}

	if len(guidance) > 0 {
		return strings.Join(guidance, "\n\n")
	}
	return ""
}

// buildCompoundTaskPrompt creates a prompt for multi-category tasks
// Task 3.3: Refactored to use detectCategory() from category_prompts.go
func (p *PromptBuilder) buildCompoundTaskPrompt(mode Mode, taskDescription string) string {
	// Use detectCategory for primary detection
	primaryCategory := detectCategory(taskDescription)
	categories := []string{primaryCategory}

	// Add related categories based on primary
	taskLower := strings.ToLower(taskDescription)

	// Detect additional categories from compound task indicators
	if strings.Contains(taskLower, " and ") || strings.Contains(taskLower, " then ") {
		// Check for additional category indicators beyond the primary
		additional := []string{}

		if strings.Contains(taskLower, "test") && primaryCategory != "test" {
			additional = append(additional, "test")
		}

		if (strings.Contains(taskLower, "deploy") || strings.Contains(taskLower, "docker")) &&
			primaryCategory != "devops" {
			additional = append(additional, "devops")
		}

		// Add related categories for the primary category
		related := getRelatedCategories(primaryCategory)
		categories = append(categories, related...)
		categories = append(categories, additional...)
	}

	// Build compound prompt with unique categories
	basePrompt := p.SystemPrompt(mode)
	compoundGuidance := p.composeCompoundGuidance(uniqueCategories(categories))

	return basePrompt + `

## COMPOUND TASK DETECTED

This task involves multiple steps across different domains. Execute them in order:

` + compoundGuidance
}

// uniqueCategories removes duplicates from a category slice
func uniqueCategories(categories []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, cat := range categories {
		if !seen[cat] && cat != "" && cat != "default" {
			seen[cat] = true
			result = append(result, cat)
		}
	}
	return result
}

// composeCompoundGuidance creates ordered guidance for compound tasks
func (p *PromptBuilder) composeCompoundGuidance(categories []string) string {
	var steps []string

	for i, cat := range categories {
		if prompt, ok := categoryPrompts[cat]; ok && cat != "default" {
			// Get first line as summary
			lines := strings.Split(prompt, "\n")
			summary := strings.TrimSpace(lines[0])
			if strings.HasPrefix(summary, "You are a") {
				summary = strings.TrimPrefix(summary, "You are a")
				summary = strings.TrimSpace(summary)
			}

			steps = append(steps, fmt.Sprintf("%d. **%s**: %s", i+1, cat, summary))
		}
	}

	return strings.Join(steps, "\n") + `

### Execution Guidelines
- Complete each phase fully before moving to the next
- Verify intermediate results before proceeding
- If a phase fails, diagnose before continuing
- Document your progress through each phase`
}

// ParseTaskForHints performs semantic parsing on task instructions
// to extract category hints and task complexity indicators
func (p *PromptBuilder) ParseTaskForHints(task string) (hints []string, complexity string) {
	taskLower := strings.ToLower(task)

	// Detect primary category
	primaryCategory := detectCategory(task)
	if primaryCategory != "default" {
		hints = append(hints, primaryCategory)
	}

	// Detect complexity
	complexityIndicators := map[string]int{
		"simple":   0,
		"basic":    0,
		"easy":     0,
		"complex":  1,
		"advanced": 1,
		"recover":  1,
		"fix":      1,
		"debug":    1,
		"multiple": 2,
		"large":    2,
	}

	complexityScore := 0
	for indicator, score := range complexityIndicators {
		if strings.Contains(taskLower, indicator) {
			complexityScore += score
		}
	}

	// Determine complexity level
	switch {
	case complexityScore >= 2:
		complexity = "high"
	case complexityScore >= 1:
		complexity = "medium"
	default:
		complexity = "low"
	}

	// Check for compound tasks
	if detectCompoundTasks(task) {
		complexity = "high"
		// Add related categories for compound tasks
		related := getRelatedCategories(primaryCategory)
		hints = append(hints, related...)
	}

	return hints, complexity
}

// Build constructs a complete prompt with system, context, and user messages.
func (p *PromptBuilder) Build(mode Mode, userInput string) string {
	return p.BuildWithAnalysis(mode, userInput)
}

// BuildWithAnalysis constructs the final prompt with automatic task analysis
func (p *PromptBuilder) BuildWithAnalysis(mode Mode, userInput string) string {
	// Parse task for hints and complexity
	hints, complexity := p.ParseTaskForHints(userInput)

	// Get appropriate system prompt
	systemPrompt := p.SystemPromptWithTask(mode, userInput)

	// Add complexity note if high
	if complexity == "high" {
		systemPrompt += `

## COMPLEXITY NOTE
This appears to be a complex multi-step task. Take your time, verify each step,
and don't rush to completion. If stuck, try a different approach.`
	}

	// Add project context
	context := GetProjectContext()
	if context != "" {
		systemPrompt = systemPrompt + "\n\n" + context
	}

	// Add hints info for debugging (can be removed in production)
	if len(hints) > 0 {
		systemPrompt += fmt.Sprintf("\n\n[Detected categories: %s]", strings.Join(hints, ", "))
	}

	return fmt.Sprintf("[SYSTEM]\n%s\n\n[USER]\n%s\n", systemPrompt, userInput)
}

// BuildWithCategory builds prompt with explicit category selection
func (p *PromptBuilder) BuildWithCategory(mode Mode, userInput string, category string) string {
	systemPrompt := p.SystemPrompt(mode, category)

	context := GetProjectContext()
	if context != "" {
		systemPrompt = systemPrompt + "\n\n" + context
	}

	return fmt.Sprintf("[SYSTEM]\n%s\n\n[USER]\n%s\n", systemPrompt, userInput)
}

// SetCategoryHints allows setting default category hints for this builder instance
func (p *PromptBuilder) SetCategoryHints(hints []string) {
	p.categoryHints = hints
}

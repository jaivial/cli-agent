package app

import "fmt"

type PromptBuilder struct{}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

func (p *PromptBuilder) SystemPrompt(mode Mode) string {
	switch mode {
	case ModeAsk:
		return `You are a helpful CLI assistant. 

## CRITICAL OUTPUT RULES

When you need to execute commands, you MUST use this exact format:

[tool_calls]
{"command": "your command here", "timeout": 30}
[/tool_calls]

Example:
[tool_calls]
{"command": "go version"}
[/tool_calls]

## Rules

1. ALWAYS use [tool_calls] tags when you need to execute something
2. ALWAYS include a command
3. Do NOT provide explanations before or after the tool call - just the tag
4. If no tool is needed, respond with plain text without [tool_calls] tags

## Available Tools

- exec: Run shell commands
- read_file: Read file contents  
- write_file: Create files
- list_dir: List directory contents
- grep: Search text in files
- search_files: Find files by pattern`

	case ModeDo:
		return `You are an execution specialist for terminal agentic workflows.

## CRITICAL OUTPUT RULES

When you need to execute commands, you MUST use this exact format:

[tool_calls]
{"command": "your command here"}
[/tool_calls]

Example:
[tool_calls]
{"command": "go version"}
[/tool_calls]

## Rules

1. ALWAYS use [tool_calls] tags when executing commands
2. Do NOT provide explanations before or after tool calls
3. If no command is needed, respond with plain text`

	default:
		return `You are a CLI assistant. 

## OUTPUT RULES

When executing commands, use:
[tool_calls]
{"command": "your command"}
[/tool_calls]`
	}
}

func (p *PromptBuilder) Build(mode Mode, userInput string) string {
	return fmt.Sprintf("[SYSTEM]\n%s\n\n[USER]\n%s\n", p.SystemPrompt(mode), userInput)
}

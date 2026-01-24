# KiloCode: Add Multiple Operating Modes to CLI Agent

## Task
Add these 4 new modes to `/Users/usuario/Desktop/cli-agent/cmd/eai/main.go` and `/Users/usuario/Desktop/cli-agent/internal/app/modes.go`:

## New Modes to Add

1. **ask** - Simple Q&A without tool execution
2. **code** - Code-focused mode with file editing
3. **debug** - Debugging mode with error analysis
4. **architect** - System design mode (currently exists as "architect" but needs better prompts)

## Implementation Steps

### 1. Update `/Users/usuario/Desktop/cli-agent/internal/app/modes.go`
Add these mode constants:
```go
ModeAsk    Mode = "ask"
ModeCode   Mode = "code" 
ModeDebug  Mode = "debug"
```

Update `ParseMode()` to handle the new modes.

### 2. Update `/Users/usuario/Desktop/cli-agent/internal/app/prompt.go`
Add system prompts for new modes:
- **ask**: "You are a helpful CLI assistant. Answer questions directly without using tools."
- **code**: "You are a code expert. Focus on file operations, code review, and modifications."
- **debug**: "You are a debugging specialist. Analyze errors and suggest fixes."

### 3. Update `/Users/usuario/Desktop/cli-agent/cmd/eai/main.go`
Add `--mode` flag options to include: ask, code, debug, architect

### 4. Update `/Users/usuario/Desktop/cli-agent/internal/app/tools.go`
Add new tools for code/debug mode:
- `edit_file` - Modify specific lines in a file
- `grep_error` - Search for error patterns in logs
- `git_status` - Check git repository status
- `run_test` - Execute test commands

## Files to Modify
1. `internal/app/modes.go` - Add mode constants and parsing
2. `internal/app/prompt.go` - Add system prompts for new modes  
3. `cmd/eai/main.go` - Update mode help text
4. `internal/app/tools.go` - Add new tools (optional)

## Requirements
- Add the 4 new modes (ask, code, debug, architect)
- Keep existing modes (plan, do, orchestrate) working
- Update system prompts for better AI behavior
- No breaking changes

## Output
Return a summary of what was changed and which files were modified.

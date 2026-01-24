# KiloCode Prompt: Enhance CLI Agent with Advanced Features

## Objective
Enhance the existing CLI agent (`eai`) in `/Users/usuario/Desktop/cli-agent` with advanced features similar to KiloCode and Terminal-Bench 2.0 compliance.

## Current State
- ✅ Basic TUI with bubbletea
- ✅ MiniMax API client integration
- ✅ Tool calling system (8 tools: exec, read_file, write_file, list_dir, search_files, grep, web_search, web_fetch)
- ✅ Agent loop with max iterations
- ✅ Mock mode for testing (--mock flag)
- ✅ 92.3% success rate on Terminal-Bench (mock mode)

## Required Improvements

### 1. Multiple Operating Modes (like KiloCode)
Add these modes to the agent:
- **ask**: Simple Q&A mode without tool execution
- **code**: Code-focused mode with file editing tools
- **debug**: Debugging mode with error analysis
- **architect**: System design mode
- **plan**: Planning mode (current default)
- **do**: Execution mode (current default)
- **orchestrate**: Multi-agent coordination mode

### 2. Parallel Execution Support
- Add `--parallel` flag to run tasks in parallel
- Create separate git branches for each parallel task
- Support `--existing-branch` flag to work on existing branches
- Implement concurrent tool execution

### 3. Enhanced Tool System
- Add file editing tool (modify existing files)
- Add git tools (status, commit, branch)
- Add docker/kubectl tools (for container operations)
- Add search/replace tool for code modifications
- Improve tool descriptions for better AI understanding

### 4. Session Management
- Add `--continue` flag to resume previous conversations
- Store conversation history in workspace
- Support `--session` and `--fork` flags for session management
- Persist agent state between sessions

### 5. Configuration & Customization
- Add `--append-system-prompt` flag
- Add `--append-system-prompt-file` flag
- Add `--on-task-completed` callback support
- Improve configuration file handling

### 6. Output Formats
- Add `--json` flag for JSON output (requires --auto)
- Add `--json-io` flag for bidirectional JSON communication
- Improve logging and progress reporting

### 7. Harbor Framework Integration (Terminal-Bench 2.0)
Create a Python adapter script for Harbor framework:
- Implement BaseAgent interface
- Create wrapper class for the CLI agent
- Support Docker sandboxed execution
- Enable proper result verification
- Generate Harbor-compatible output

### 8. Documentation
- Update README with new features
- Add usage examples for each mode
- Document Harbor integration
- Add troubleshooting guide

## Technical Requirements

### Code Structure
```
cmd/eai/
  main.go          # Main entry point with all commands
internal/app/
  agent.go         # Agent loop and tool execution
  modes.go         # Mode definitions and parsing
  tools.go         # Enhanced tool definitions
  session.go       # Session management
  harbor_adapter.py # Harbor framework adapter
internal/tui/
  app.go           # TUI with mode selection
```

### Implementation Priority
1. **Must have (80% value):**
   - Enhanced modes (ask, code, debug, architect, plan, do, orchestrate)
   - Harbor adapter for Terminal-Bench 2.0
   - Tool improvements (file editing, git)
   
2. **Should have (15% value):**
   - Parallel execution
   - Session management
   - JSON output formats
   
3. **Nice to have (5% value):**
   - Custom system prompts
   - Callbacks

## Testing Requirements
1. Ensure 70%+ success rate on Terminal-Bench 2.0
2. Test all new modes manually
3. Test parallel execution
4. Test Harbor adapter integration
5. Verify backwards compatibility

## Success Criteria
- ✅ All new modes implemented and working
- ✅ Terminal-Bench 2.0: 70%+ success rate (real mode, not mock)
- ✅ Harbor framework integration complete
- ✅ Documentation updated
- ✅ No breaking changes to existing functionality

## Output
Provide:
1. Updated Go code with all improvements
2. Python Harbor adapter script
3. Updated documentation
4. Test results

Return ONLY the file paths and brief descriptions of what was changed. Do not include code in the response - just tell me which files were modified and what was added.

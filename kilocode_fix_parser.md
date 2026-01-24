# CRITICAL: Fix MiniMax API Parser for CLI Agent

## Problem
The MiniMax API returns tool calls in **5+ different formats**, and the parser only handles some of them. The API is inconsistent - even the same task returns different formats on different calls.

## Required: Fix parseToolCalls() in /Users/usuario/Desktop/cli-agent/internal/app/agent.go

### All Response Formats to Handle:

**Format 1: Perl hash syntax with `=>`**
```
[TOOL_CALL]
{tool => "search_files", args => {
  --pattern "*.go"
}}
[/TOOL_CALL]
```
Fix: Parse `tool => "name"` and `--key "value"` patterns with regex.

**Format 2: JSON format with [TOOL_CALL]**
```
[TOOL_CALL]
{"tool": "exec", "args": {"command": "go version"}}
[/TOOL_CALL]
```
Fix: Already works, but ensure it handles nested args.

**Format 3: JSON format with [tool_calls]**
```
[tool_calls]
{"command": "go version", "timeout": 10}
[/tool_calls]
```
Note: No "tool" name - use "exec" as default.

**Format 4: Tool name as key**
```
[tool_calls]
{"list_dir": {"path": "."}}
[/tool_calls]
```
Fix: Parse `{"tool_name": {"arg1": value1}}` format.

**Format 5: Tool name in [tool_calls] with name/args**
```
[tool_calls]
{"name": "write_file", "args": {"path": "test.txt", "content": "Hello"}}
[/tool_calls]
```
Fix: Extract "name" and "args" fields.

**Format 6: Plain text (no tool call tags!)**
The API sometimes returns plain text like:
"I'll help you with that. Let me run 'go version' for you."
This contains a command but NO [TOOL_CALL] or [tool_calls] tags!

### Critical Fixes Needed:

1. **Add regex parsing for Perl syntax** (`=>`, `--key "value"`)
2. **Handle Format 3** - no tool name, default to "exec"
3. **Handle Format 4** - tool name as JSON key
4. **Handle Format 5** - name/args structure
5. **Handle Format 6** - Plain text with commands inside

### Current Test Cases That Fail:
1. "Check Go version" - sometimes plain text
2. "List files in current directory" - sometimes plain text
3. "Read the contents of go.mod" - varies
4. "Create a test.txt file with Hello World" - Perl syntax
5. "Search for func in all .go files" - varies
6. "Find all .go files" - Perl syntax

### Your Task:
1. Update parseToolCalls() function to handle ALL 6 formats
2. Test with: `cd /Users/usuario/Desktop/cli-agent && python3 parser_test_loop.py`
3. Repeat until 100% success rate
4. Do NOT ask me for permission - just fix issues as they arise

### Important Notes:
- The API is non-deterministic - same task can return different formats
- You MUST handle all formats in the same parseToolCalls() function
- Use regex for Perl syntax parsing
- Return empty toolCalls slice if truly no valid format found (don't crash)
- Each tool call MUST have: ID, Name, Arguments (valid JSON)

### Success Criteria:
- 6/6 tests passing (100% success rate)
- All formats handled correctly
- No crashes on invalid input

Go ahead and fix this. Test, fix, repeat until 100%.

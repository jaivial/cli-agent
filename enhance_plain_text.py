#!/usr/bin/env python3
import re

with open('internal/app/agent.go', 'r') as f:
    content = f.read()

# The current Format 4 plain text section
old = '''	// === Format 4: Plain text with command - fallback ===
	// Look for common command patterns in plain text
	commandPatterns := []string{
		`go version`,
		`ls `,
		`echo `,
		`cat `,
		`find `,
		`grep `,
	}
	for _, pattern := range commandPatterns {
		if strings.Contains(response, pattern) {
			// Extract the command
			cmdMatch := regexp.MustCompile(`'` + pattern + `[^']*'`).FindString(response)
			if cmdMatch == "" {
				cmdMatch = regexp.MustCompile(pattern + `[^\s'"` + "`" + `]+`).FindString(response)
			}
			if cmdMatch != "" {
				toolCalls = append(toolCalls, ToolCall{
					ID:       "exec_1",
					Name:     "exec",
					Arguments: json.RawMessage(fmt.Sprintf(`{"command": "%s"}`, cmdMatch)),
				})
				return toolCalls
			}
		}
	}
'''

new = '''	// === Format 4: Plain text with command - fallback ===
	// Look for common command patterns in plain text
	commandPatterns := []string{
		`go version`,
		`go build`,
		`ls -la`,
		`ls -l`,
		`ls `,
		`echo `,
		`cat `,
		`find `,
		`grep `,
		`touch `,
		`mkdir -p`,
	}
	for _, pattern := range commandPatterns {
		if strings.Contains(response, pattern) {
			// Extract the command
			cmdMatch := regexp.MustCompile(`'` + pattern + `[^']*'`).FindString(response)
			if cmdMatch == "" {
				cmdMatch = regexp.MustCompile(pattern + `[^\s'"` + "`" + `]+`).FindString(response)
			}
			if cmdMatch == "" {
				cmdMatch = pattern
			}
			if cmdMatch != "" {
				toolCalls = append(toolCalls, ToolCall{
					ID:       "exec_1",
					Name:     "exec",
					Arguments: json.RawMessage(fmt.Sprintf(`{"command": "%s"}`, cmdMatch)),
				})
				return toolCalls
			}
		}
	}
	
	// === Format 4b: Additional patterns for common commands ===
	// Match "I'll run/go version" patterns
	runMatch := regexp.MustCompile(`(?:run|execute)\s+(?:the\s+)?(?:command\s+)?["']?([a-zA-Z0-9\s\-./_]+?)["']?(?:\s|$|\\.)`).FindStringSubmatch(response)
	if len(runMatch) > 1 {
		cmd := strings.TrimSpace(runMatch[1])
		if len(cmd) > 2 && len(cmd) < 100 {
			toolCalls = append(toolCalls, ToolCall{
				ID:       "exec_1",
				Name:     "exec",
				Arguments: json.RawMessage(fmt.Sprintf(`{"command": "%s"}`, cmd)),
			})
			return toolCalls
		}
	}
'''

if old in content:
    content = content.replace(old, new)
    with open('internal/app/agent.go', 'w') as f:
        f.write(content)
    print('Enhanced plain text extraction')
else:
    print('Pattern not found - trying alternative approach')
    # Try alternative - find by line numbers
    lines = content.split('\n')
    for i, line in enumerate(lines):
        if 'Format 4: Plain text' in line:
            print(f'Found at line {i+1}')
            print('---')
            print('\\n'.join(lines[i:i+20]))
            print('---')

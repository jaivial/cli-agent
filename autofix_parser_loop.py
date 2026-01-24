#!/usr/bin/env python3
"""
Autonomous Parser Fixer Loop - YOLO Mode
Keeps fixing parseToolCalls() until 100% success. No asking for permission.
"""

import subprocess
import re
import os
import sys
import json
import time

AGENT_PATH = "/Users/usuario/Desktop/cli-agent"
API_KEY = "sk-cp-LOdx3q4oeKupQ7XIIYTjuoxBNDnzIBCMFy0UBMEFzT5_E1bC5-oUJiJFli0Kf4hTZuLfZzmuh8CscOSooK8wE1b3tp6uiVUsaehrWjQZ1eD6YPmxXtLhGBU"
AGENT_GO = f"{AGENT_PATH}/internal/app/agent.go"

def run_parser_test():
    """Run parser test and get output."""
    result = subprocess.run(
        f"cd {AGENT_PATH} && export MINIMAX_API_KEY='{API_KEY}' && timeout 180 python3 parser_test_loop.py 2>&1",
        shell=True,
        capture_output=True,
        timeout=200
    )
    output = result.stdout.decode() + result.stderr.decode()
    
    match = re.search(r'Success Rate:\s*(\d+\.?\d*)%\s*\((\d+)/(\d+)\)', output)
    if match:
        return float(match.group(1)), int(match.group(2)), int(match.group(3)), output
    return 0.0, 0, 6, output

def get_failure_analysis(output):
    """Analyze what failed and why."""
    analysis = []
    
    lines = output.split('\n')
    for line in lines:
        if 'Plain command text:' in line:
            analysis.append(('plain_text', line))
        elif '"type":' in line and 'Found:' in line:
            type_match = re.search(r"Found:\s*\[?'([^']+)'?\]?", line)
            if type_match:
                analysis.append(('type_field', type_match.group(1)))
        elif '[tool_calls]' in line or '[TOOL_CALL]' in line:
            json_match = re.search(r'\{[^}]+\}', line)
            if json_match:
                analysis.append(('tagged_json', json_match.group(0)))
    
    return analysis

def add_plain_text_fallback():
    """Add a robust plain text command extraction fallback."""
    with open(AGENT_GO, 'r') as f:
        content = f.read()
    
    insert_marker = '// === Format 4: {"type": "...", "parameters": {...}} ==='
    
    fallback_code = '''	// === Format 3b: Plain text with backtick commands ===
	backtickMatch := regexp.MustCompile("`([^`]+)`").FindStringSubmatch(response)
	if len(backtickMatch) > 1 {
		cmd := strings.TrimSpace(backtickMatch[1])
		toolCalls = append(toolCalls, ToolCall{
			ID:       "exec_1",
			Name:     "exec",
			Arguments: json.RawMessage(fmt.Sprintf(`{"command": "%s"}`, cmd)),
		})
		return toolCalls
	}
	
'''
    
    if insert_marker in content:
        content = content.replace(insert_marker, fallback_code + insert_marker)
        with open(AGENT_GO, 'w') as f:
            f.write(content)
        return True
    return False

def add_type_field_parser():
    """Add parser for {"path": "...", "type": "list_dir"} format."""
    with open(AGENT_GO, 'r') as f:
        content = f.read()
    
    insert_marker = '// Try {"command": "...", "type": "...", ...}'
    new_code = '''	// Try {"command": "...", "type": "..."} or {"path": "...", "type": "..."}
			var cmdTypeResp struct {
				Command string `json:"command"`
				Path    string `json:"path"`
				Type    string `json:"type"`
			}
			if err := json.Unmarshal([]byte(content), &cmdTypeResp); err == nil && cmdTypeResp.Type != "" {
				args := make(map[string]interface{})
				if cmdTypeResp.Command != "" {
					args["command"] = cmdTypeResp.Command
				}
				if cmdTypeResp.Path != "" {
					args["path"] = cmdTypeResp.Path
				}
				arguments, _ := json.Marshal(args)
				toolCalls = append(toolCalls, ToolCall{
					ID:       fmt.Sprintf("%s_1", cmdTypeResp.Type),
					Name:     cmdTypeResp.Type,
					Arguments: arguments,
				})
				return toolCalls
			}
'''
    
    if insert_marker in content:
        content = content.replace(insert_marker, new_code + insert_marker)
        with open(AGENT_GO, 'w') as f:
            f.write(content)
        return True
    return False

def build_and_test():
    """Build and test."""
    build = subprocess.run(
        f"cd {AGENT_PATH} && go build -o bin/eai ./cmd/eai 2>&1",
        shell=True,
        capture_output=True
    )
    if build.returncode != 0:
        return False, 0.0, f"Build failed"
    
    rate, passed, total, _ = run_parser_test()
    return True, rate, f"{passed}/{total}"

def main():
    print("=" * 70)
    print("AUTONOMOUS PARSER FIXER - YOLO MODE")
    print("=" * 70)
    
    iteration = 0
    max_iters = 100
    applied_fixes = set()
    
    while iteration < max_iters:
        iteration += 1
        print(f"\nIteration {iteration}/{max_iters}")
        
        success, rate, result = build_and_test()
        
        if not success:
            print(f"Build failed - stopping")
            break
        
        print(f"Result: {rate:.1f}% ({result})")
        
        if rate >= 100:
            print("\nPERFECT! 100% success!")
            break
        
        _, _, _, output = run_parser_test()
        failures = get_failure_analysis(output)
        
        fixes_this_round = 0
        
        for fail_type, detail in failures:
            if fail_type == 'plain_text' and 'plain_text_fallback' not in applied_fixes:
                if add_plain_text_fallback():
                    applied_fixes.add('plain_text_fallback')
                    print("  + Added plain text fallback")
                    fixes_this_round += 1
            
            elif fail_type == 'type_field' and f'type_field_{detail}' not in applied_fixes:
                if add_type_field_parser():
                    applied_fixes.add(f'type_field_{detail}')
                    print(f"  + Added type field parser for: {detail}")
                    fixes_this_round += 1
        
        if fixes_this_round == 0 and rate < 100:
            print("  - No automatic fixes - trying KiloCode...")
            result = subprocess.run(
                f"cd {AGENT_PATH} && kilocode --auto --yolo --nosplash 'Fix parseToolCalls() to handle all MiniMax API formats. The API returns inconsistent formats. Make parser handle ALL cases. Test until 100% success.' 2>&1",
                shell=True,
                timeout=180
            )
            if result.returncode == 0:
                print("  + KiloCode applied fixes")
    
    print("\n" + "=" * 70)
    print(f"Completed after {iteration} iterations")
    print(f"Fixes applied: {len(applied_fixes)}")
    print("=" * 70)

if __name__ == "__main__":
    main()

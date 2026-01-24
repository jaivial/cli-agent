#!/usr/bin/env python3
"""
MiniMax API Parser Testing Loop

This script:
1. Tests the CLI agent with real MiniMax API
2. Captures responses that fail parsing
3. Analyzes the response format
4. Reports parsing issues
5. Generates test cases for each format
"""

import subprocess
import json
import re
import os
from datetime import datetime

# Test cases that trigger different parsing scenarios
TEST_CASES = [
    {
        "name": "Simple exec command",
        "task": "Check Go version",
        "expected_tool": "exec",
        "expected_args": {"command": "go version"}
    },
    {
        "name": "List directory",
        "task": "List files in current directory",
        "expected_tool": "list_dir",
        "expected_args": {"path": "."}
    },
    {
        "name": "Read file",
        "task": "Read the contents of go.mod",
        "expected_tool": "read_file",
        "expected_args": {"path": "go.mod"}
    },
    {
        "name": "Write file",
        "task": "Create a test.txt file with 'Hello World'",
        "expected_tool": "write_file",
        "expected_args": {"path": "test.txt", "content": "Hello World"}
    },
    {
        "name": "Grep search",
        "task": "Search for 'func' in all .go files",
        "expected_tool": "grep",
        "expected_args": {"pattern": "func", "path": ".", "recursive": True}
    },
    {
        "name": "Search files",
        "task": "Find all .go files",
        "expected_tool": "search_files",
        "expected_args": {"pattern": "*.go"}
    },
]

def run_test(task_data):
    """Run a single test and capture the response."""
    task = task_data["task"]
    print(f"\n[{len(TEST_CASES)+1}] Testing: {task}")
    
    result = subprocess.run(
        ["./bin/eai", "agent", task],
        capture_output=True,
        text=True,
        timeout=60,
        env={"MINIMAX_API_KEY": os.environ.get("MINIMAX_API_KEY", "")}
    )
    
    return {
        "stdout": result.stdout,
        "stderr": result.stderr,
        "returncode": result.returncode,
        "task": task,
        "expected_tool": task_data["expected_tool"]
    }

def analyze_response(response_text):
    """Analyze a response and extract potential tool calls."""
    formats_found = []
    
    # Format 1: [TOOL_CALL]{"tool": "...", "args": {...}}[/TOOL_CALL]
    if "[TOOL_CALL]" in response_text:
        match = re.search(r'\[TOOL_CALL\](.*?)\[/TOOL_CALL\]', response_text, re.DOTALL)
        if match:
            formats_found.append({
                "format": "[TOOL_CALL]...[/TOOL_CALL]",
                "content": match.group(1).strip()
            })
    
    # Format 2: [tool_calls:{"name": ...}]
    if "[tool_calls:" in response_text:
        match = re.search(r'\[tool_calls:(.*?)\]', response_text, re.DOTALL)
        if match:
            formats_found.append({
                "format": "[tool_calls:...]",
                "content": match.group(1).strip()
            })
    
    # Format 3: [tool_call:{"name": ...}]
    if "[tool_call:" in response_text:
        match = re.search(r'\[tool_call:(.*?)\]', response_text, re.DOTALL)
        if match:
            formats_found.append({
                "format": "[tool_call:...]",
                "content": match.group(1).strip()
            })
    
    # Format 4: [tool_calls]{"tool": ...}[/tool_calls]
    if "[tool_calls]" in response_text and "[/tool_calls]" in response_text:
        match = re.search(r'\[tool_calls\](.*?)\[/tool_calls\]', response_text, re.DOTALL)
        if match:
            formats_found.append({
                "format": "[tool_calls]...[/tool_calls]",
                "content": match.group(1).strip()
            })
    
    # Format 5: JSON object with "tool" key
    tool_matches = re.findall(r'"tool"\s*:\s*"([^"]+)"', response_text)
    if tool_matches:
        formats_found.append({
            "format": '"tool": "..."',
            "content": f"Found: {tool_matches}"
        })
    
    # Format 6: JSON object with "type" key (for exec, list_dir, etc.)
    type_matches = re.findall(r'"type"\s*:\s*"([^"]+)"', response_text)
    if type_matches:
        formats_found.append({
            "format": '"type": "..."',
            "content": f"Found: {type_matches}"
        })
    
    # Format 7: Plain command detection
    if "go version" in response_text or "ls " in response_text or "echo " in response_text:
        formats_found.append({
            "format": "Plain command text",
            "content": "Contains shell command text"
        })
    
    return formats_found

def test_parser_format(raw_json):
    """Test if our Go parser can handle this format."""
    # Simulate what the Go parser does
    issues = []
    
    # Check if it's a valid JSON
    try:
        data = json.loads(raw_json)
    except json.JSONDecodeError as e:
        issues.append(f"Invalid JSON: {e}")
        return issues
    
    # Check for required fields
    if "tool" not in data and "name" not in data and "type" not in data:
        issues.append("Missing tool/name/type field")
    
    # Check if tool name is empty
    tool_name = data.get("tool") or data.get("name") or data.get("type", "")
    if not tool_name:
        issues.append(f"Empty tool name (data: {data})")
    
    return issues

def main():
    os.chdir("/Users/usuario/Desktop/cli-agent")
    
    print("=" * 60)
    print("MiniMax API Parser Testing Loop")
    print("=" * 60)
    print(f"Started: {datetime.now().isoformat()}")
    print()
    
    # Run all tests
    all_results = []
    all_formats = []
    
    for i, test_case in enumerate(TEST_CASES, 1):
        print(f"\n{i}/{len(TEST_CASES)} Testing: {test_case['name']}")
        
        result = run_test(test_case)
        all_results.append(result)
        
        # Analyze the response
        formats = analyze_response(result["stdout"])
        all_formats.extend(formats)
        
        # Check if tools were executed
        if "Iterations: 1" in result["stdout"] or "Tools executed: 1" in result["stdout"]:
            print(f"  ✅ Tool executed successfully")
        elif "Unknown tool:" in result["stdout"]:
            print(f"  ❌ Parsing failed - unknown tool")
        else:
            print(f"  ⚠️  Response received, needs analysis")
    
    # Report findings
    print("\n" + "=" * 60)
    print("PARSING ISSUES REPORT")
    print("=" * 60)
    
    # Unique formats found
    print("\n📊 Response Formats Found:")
    for fmt in all_formats:
        print(f"  - {fmt['format']}: {fmt['content'][:80]}")
    
    # Count successes
    successes = sum(1 for r in all_results if "Iterations: 1" in r["stdout"])
    rate = successes / len(all_results) * 100
    
    print(f"\n📈 Success Rate: {rate:.1f}% ({successes}/{len(all_results)})")
    
    if rate >= 100:
        print("\n🎉 ALL TESTS PASSED! No parsing issues.")
        return 0
    else:
        print(f"\n⚠️  {len(all_results) - successes} test(s) failed.")
        print("\n🔧 Parser needs fixes for:")
        
        for i, result in enumerate(all_results, 1):
            if "Unknown tool:" in result["stdout"]:
                print(f"\n  Test {i}: {result['task']}")
                formats = analyze_response(result["stdout"])
                for fmt in formats:
                    print(f"    - {fmt['format']}")
        
        print("\n💡 Next steps:")
        print("  1. Update parseToolCalls() in agent.go to handle these formats")
        print("  2. Run this script again to verify fixes")
        print("  3. Repeat until 100% success rate")
        
        return 1

if __name__ == "__main__":
    exit(main())

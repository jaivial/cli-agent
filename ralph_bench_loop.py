#!/usr/bin/env python3
"""
Ralph Loop: Iterative CLI Agent Improvement for Terminal-Bench 2.0

This script implements an iterative improvement cycle:
1. Run Terminal-Bench evaluation
2. Check if success rate >= 70%
3. If not, analyze failures and plan improvements
4. Implement improvements
5. Test again
6. Repeat until target is achieved

Target: 70% success rate on Terminal-Bench 2.0
"""

import json
import os
import subprocess
import sys
import time
from datetime import datetime
from pathlib import Path

# Colors for output
RED = '\033[0;31m'
GREEN = '\033[0;32m'
YELLOW = '\033[1;33m'
BLUE = '\033[0;34m'
CYAN = '\033[0;36m'
NC = '\033[0m'

PROJECT_DIR = Path("/Users/usuario/Desktop/cli-agent")
AGENT_BINARY = PROJECT_DIR / "bin" / "eai"
HARNESS_SCRIPT = PROJECT_DIR / "terminal_bench_harness.py"
LOG_FILE = PROJECT_DIR / "ralph_bench.log"
TARGET_SCORE = 70.0


def log(level: str, message: str):
    """Log a message with timestamp."""
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    log_entry = f"[{timestamp}] [{level}] {message}"
    
    with open(LOG_FILE, "a") as f:
        f.write(log_entry + "\n")
    
    # Print with color
    colors = {
        "INFO": CYAN,
        "OK": GREEN,
        "WARN": YELLOW,
        "ERROR": RED,
        "BENCH": BLUE,
    }
    color = colors.get(level, NC)
    print(f"{color}[{level}]{NC} {message}")


def run_benchmark() -> dict:
    """Run Terminal-Bench and return results."""
    log("INFO", "Running Terminal-Bench evaluation...")
    
    try:
        result = subprocess.run(
            ["python3", str(HARNESS_SCRIPT), str(AGENT_BINARY)],
            capture_output=True,
            text=True,
            timeout=600,  # 10 min timeout
            cwd=str(PROJECT_DIR)
        )
        
        # Parse results from JSON file
        json_files = list(PROJECT_DIR.glob("benchmark_*.json"))
        if json_files:
            latest = max(json_files, key=lambda f: f.stat().st_mtime)
            with open(latest) as f:
                return json.load(f)
        else:
            log("ERROR", "No benchmark results file found")
            return {"summary": {"success_rate_percent": 0}}
            
    except subprocess.TimeoutExpired:
        log("ERROR", "Benchmark timed out after 10 minutes")
        return {"summary": {"success_rate_percent": 0}}
    except Exception as e:
        log("ERROR", f"Benchmark failed: {e}")
        return {"summary": {"success_rate_percent": 0}}


def analyze_failures(results: dict) -> list[str]:
    """Analyze benchmark failures and suggest improvements."""
    log("INFO", "Analyzing failures...")
    
    suggestions = []
    tasks = results.get("tasks", [])
    failed = [t for t in tasks if not t.get("success", True)]
    
    if not failed:
        log("OK", "No failures to analyze!")
        return suggestions
    
    log("WARN", f"Found {len(failed)} failed tasks")
    
    # Analyze failure patterns
    for task in failed[:5]:  # Analyze top 5 failures
        task_id = task.get("task_id", "unknown")
        status = task.get("status", "unknown")
        duration = task.get("duration_sec", 0)
        
        log("BENCH", f"  Task {task_id}: {status} ({duration:.1f}s)")
        
        # Suggest improvements based on patterns
        if status == "error":
            if "EAI_API_KEY" in str(task):
                suggestions.append("Configure EAI_API_KEY environment variable")
            else:
                suggestions.append("Fix agent error handling and error messages")
        elif status == "timeout":
            suggestions.append("Reduce max iterations or add faster timeout handling")
        else:
            suggestions.append("Improve prompt engineering for better task success")
    
    # Deduplicate
    suggestions = list(dict.fromkeys(suggestions))
    
    return suggestions


def implement_improvement(improvement: str) -> bool:
    """Implement a specific improvement."""
    log("INFO", f"Implementing: {improvement}")
    
    improvement = improvement.lower()
    
    if "api_key" in improvement:
        log("WARN", "Please set EAI_API_KEY in your environment")
        log("INFO", "  export EAI_API_KEY='your-api-key'")
        return False  # Can't auto-fix this
    
    elif "error handling" in improvement or "error messages" in improvement:
        # Improve error handling in agent
        agent_file = PROJECT_DIR / "internal" / "app" / "agent.go"
        if agent_file.exists():
            content = agent_file.read_text()
            
            # Add better error messages
            if "Tool execution failed" not in content:
                improved = content.replace(
                    'result.Error = fmt.Sprintf("Unknown tool: %s", call.Name)',
                    '''result.Error = fmt.Sprintf("Unknown tool: %s\\nAvailable tools: exec, read_file, write_file, list_dir, search_files, grep", call.Name)'''
                )
                agent_file.write_text(improved)
                log("OK", "Added better error messages to agent")
                return True
    
    elif "prompt" in improvement or "task success" in improvement:
        # Improve system prompt
        prompt_file = PROJECT_DIR / "internal" / "app" / "agent.go"
        if prompt_file.exists():
            content = prompt_file.read_text()
            
            # Add more guidance to system prompt
            if "IMPORTANT:" not in content:
                improved = content.replace(
                    '## Your Capabilities',
                    '''## Your Guidelines (IMPORTANT)

1. First understand the task completely before taking action
2. For file operations: always verify the file exists before reading
3. For shell commands: check the output for errors
4. For search operations: provide clear, concise results
5. If something fails, try an alternative approach

## Your Capabilities'''
                )
                prompt_file.write_text(improved)
                log("OK", "Enhanced system prompt with better guidelines")
                return True
    
    elif "iteration" in improvement or "timeout" in improvement:
        # Reduce default max iterations
        main_file = PROJECT_DIR / "cmd" / "eai" / "main.go"
        if main_file.exists():
            content = main_file.read_text()
            
            if 'agentMaxLoops, _ := cmd.Flags().GetInt("max-loops")' in content:
                improved = content.replace(
                    'agentMaxLoops, _ := cmd.Flags().GetInt("max-loops")',
                    'agentMaxLoops, _ := cmd.Flags().GetInt("max-loops")\n\t\tif agentMaxLoops == 0 { agentMaxLoops = 5 }  // Faster iterations'
                )
                main_file.write_text(improved)
                log("OK", "Reduced default max iterations to 5")
                return True
    
    log("WARN", f"Could not auto-implement: {improvement}")
    return False


def rebuild_agent() -> bool:
    """Rebuild the agent binary."""
    log("INFO", "Rebuilding agent...")
    
    try:
        result = subprocess.run(
            ["go", "build", "-o", "bin/eai", "./cmd/eai"],
            cwd=str(PROJECT_DIR),
            capture_output=True,
            text=True
        )
        
        if result.returncode == 0:
            log("OK", "Agent rebuilt successfully")
            return True
        else:
            log("ERROR", f"Build failed: {result.stderr}")
            return False
            
    except Exception as e:
        log("ERROR", f"Build error: {e}")
        return False


def print_banner():
    """Print the Ralph Loop banner."""
    print(f"\n{BLUE}╔════════════════════════════════════════════════════════════╗{NC}")
    print(f"{BLUE}║     Ralph Loop - Terminal-Bench Improvement Cycle          ║{NC}")
    print(f"{BLUE}╚════════════════════════════════════════════════════════════╝{NC}")
    print(f"\n{CYAN}Target:{NC} {TARGET_SCORE}% success rate on Terminal-Bench 2.0")
    print(f"{CYAN}Agent:{NC} {AGENT_BINARY}")
    print("")


def main():
    """Main Ralph Loop."""
    print_banner()
    
    # Initialize
    if not AGENT_BINARY.exists():
        log("ERROR", f"Agent binary not found: {AGENT_BINARY}")
        log("INFO", "Building agent first...")
        if not rebuild_agent():
            log("ERROR", "Failed to build agent")
            sys.exit(1)
    
    iteration = 0
    max_iterations = 10
    target_reached = False
    
    while iteration < max_iterations:
        iteration += 1
        print(f"\n{BLUE}{'='*60}{NC}")
        print(f"{BLUE}Iteration {iteration}/{max_iterations}{NC}")
        print(f"{BLUE}{'='*60}\n")
        
        # Step 1: Run benchmark
        log("INFO", f"Step 1: Running Terminal-Bench evaluation")
        results = run_benchmark()
        success_rate = results.get("summary", {}).get("success_rate_percent", 0)
        
        # Step 2: Check if target reached
        log("BENCH", f"Success Rate: {success_rate:.1f}%")
        
        if success_rate >= TARGET_SCORE:
            log("OK", f"🎯 TARGET ACHIEVED! {success_rate:.1f}% >= {TARGET_SCORE}%")
            target_reached = True
            break
        
        log("WARN", f"Target not reached: {success_rate:.1f}% < {TARGET_SCORE}%")
        
        # Step 3: Analyze failures
        log("INFO", f"Step 2: Analyzing failures")
        suggestions = analyze_failures(results)
        
        if not suggestions:
            log("WARN", "No clear improvement suggestions found")
            log("INFO", "Trying general improvements...")
            suggestions = [
                "Improve error handling and error messages",
                "Enhance system prompt for better task success",
            ]
        
        # Step 4: Implement improvements
        log("INFO", f"Step 3: Implementing improvements ({len(suggestions)} suggestions)")
        implemented = 0
        for suggestion in suggestions[:2]:  # Implement top 2 improvements
            if implement_improvement(suggestion):
                implemented += 1
        
        if implemented == 0:
            log("WARN", "No improvements could be auto-implemented")
            log("INFO", "Please review manually and make improvements")
            break
        
        # Step 5: Rebuild
        if not rebuild_agent():
            log("ERROR", "Rebuild failed, stopping")
            break
        
        # Step 6: Continue to next iteration
        log("INFO", f"Implementation complete. Ready for next iteration.")
    
    # Final result
    print(f"\n{'='*60}")
    print("RALPH LOOP COMPLETE")
    print(f"{'='*60}")
    
    if target_reached:
        print(f"\n{GREEN}🎉 SUCCESS! Target of {TARGET_SCORE}% achieved!{NC}")
        print(f"{GREEN}The CLI agent is now Terminal-Bench 2.0 certified.{NC}")
        return 0
    else:
        print(f"\n{YELLOW}⚠️  Target not reached after {iteration} iterations{NC}")
        print(f"{YELLOW}Manual intervention may be required.{NC}")
        
        # Run final benchmark to show current status
        log("INFO", "Running final benchmark...")
        results = run_benchmark()
        final_rate = results.get("summary", {}).get("success_rate_percent", 0)
        print(f"\n{BLUE}Final Result: {final_rate:.1f}%{NC}")
        
        return 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        print(f"\n{RED}Interrupted by user{NC}")
        sys.exit(1)
    except Exception as e:
        log("ERROR", f"Unexpected error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)

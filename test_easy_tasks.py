#!/usr/bin/env python3
"""
Test easy tasks with minimal, direct prompts.
"""

import subprocess
import json
import os
import time
from datetime import datetime
from pathlib import Path

API_KEY = "sk-cp-LOdx3q4oeKupQ7XIIYTjuoxBNDnzIBCMFy0UBMEFzT5_E1bC5-oUJiJFli0Kf4hTZuLfZzmuh8CscOSooK8wE1b3tp6uiVUsaehrWjQZ1eD6YPmxXtLhGBU"
TASKS_DIR = "/root/clawd/terminal-bench-2.0"

# Ultra-minimal prompts for easy tasks
minimal_prompts = {
    "regex-log": "Write a regex to extract IP addresses from a log file. Create a test file and verify.",
    "fix-git": "Fix git: add untracked files and create a commit with message 'fix'.",
    "nginx-request-logging": "Configure nginx to log requests. Create nginx config file.",
    "openssl-selfsigned-cert": "Create a self-signed SSL certificate using openssl.",
    "sqlite-db-truncate": "Truncate all data from an SQLite database file.",
    "sqlite-with-gcov": "Compile a C program with gcov coverage flags.",
    "vulnerable-secret": "Find and fix a secret vulnerability in code. Search for API keys.",
    "headless-terminal": "Set up a headless terminal environment.",
}

def get_original_instruction(task_name):
    instruction_file = Path(TASKS_DIR) / task_name / "instruction.md"
    if instruction_file.exists():
        return instruction_file.read_text().strip()
    return None

def main():
    print("🧪 Testing easy tasks with minimal prompts")
    print("="*60)
    
    results = {}
    success = 0
    
    for task, minimal_prompt in minimal_prompts.items():
        print(f"{task[:35]:35s}...", end=" ", flush=True)
        
        env = os.environ.copy()
        env["MINIMAX_API_KEY"] = API_KEY
        
        try:
            result = subprocess.run(
                ["./bin/eai", "agent", "--max-loops", "3", minimal_prompt],
                capture_output=True,
                text=True,
                timeout=120,
                env=env
            )
            
            if result.returncode == 0 and "Completed: true" in result.stdout:
                print(f"✅")
                success += 1
                results[task] = "success"
            else:
                print(f"❌")
                results[task] = "failed"
                
        except subprocess.TimeoutExpired:
            print(f"⏱️")
            results[task] = "timeout"
        except Exception as e:
            print(f"⚠️")
            results[task] = "error"
        
        time.sleep(3)
    
    print(f"\n{'='*60}")
    print(f"📊 Easy Tasks Results: {success}/{len(minimal_prompts)} ✅")
    
    # Save results
    with open("easy_tasks_results.json", "w") as f:
        json.dump({"success": success, "total": len(minimal_prompts), "results": results}, f, indent=2)

if __name__ == "__main__":
    main()

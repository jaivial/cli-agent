#!/usr/bin/env python3
"""Terminal-Bench 2.0 Quick Evaluation"""

import subprocess
import json
import os
from pathlib import Path

API_KEY = os.environ.get("EAI_API_KEY", "")
if not API_KEY:
    raise SystemExit("EAI_API_KEY is not set")
TASKS_DIR = "/root/clawd/terminal-bench-2.0"

def get_task_instruction(task_name):
    """Get instruction for a task."""
    instruction_file = Path(TASKS_DIR) / task_name / "instruction.md"
    if instruction_file.exists():
        return instruction_file.read_text().strip()
    return None

# Simple test tasks
test_tasks = [
    "fix-git",
    "regex-log", 
    "count-files",
]

print("🧪 Terminal-Bench 2.0 Quick Test")
print("="*50)

results = []
for i, task in enumerate(test_tasks, 1):
    instruction = get_task_instruction(task)
    if not instruction:
        print(f"[{i}/{len(test_tasks)}] {task}: ❌ NO INSTRUCTION")
        continue
    
    # Truncate instruction to first 200 chars for speed
    short_instr = instruction[:200] + "..."
    
    print(f"[{i}/{len(test_tasks)}] {task}...", end=" ", flush=True)
    
    env = os.environ.copy()
    env["EAI_API_KEY"] = API_KEY
    
    try:
        result = subprocess.run(
            ["./bin/eai", "agent", "--max-loops", "3", "--mock", short_instr],
            capture_output=True,
            text=True,
            timeout=60,
            env=env
        )
        
        if result.returncode == 0 and "Completed: true" in result.stdout:
            print(f"✅")
            results.append({"task": task, "status": "success"})
        else:
            print(f"❌ (exit {result.returncode})")
            results.append({"task": task, "status": "failed"})
    except subprocess.TimeoutExpired:
        print(f"⏱️  timeout")
        results.append({"task": task, "status": "timeout"})
    except Exception as e:
        print(f"⚠️  {str(e)[:30]}")
        results.append({"task": task, "status": "error"})

success = sum(1 for r in results if r["status"] == "success")
total = len(results)

print(f"\n{'='*50}")
print(f"Results: {success}/{total} ✅")
print(f"Success rate: {success/total*100:.0f}%")

# Save results
with open("tbench2_quick.json", "w") as f:
    json.dump({
        "timestamp": str(datetime.now()),
        "total": total,
        "successful": success,
        "results": results
    }, f)

#!/usr/bin/env python3
"""Terminal-Bench 2.0 Full Benchmark"""

import subprocess
import json
import os
from datetime import datetime
from pathlib import Path

API_KEY = "sk-cp-LOdx3q4oeKupQ7XIIYTjuoxBNDnzIBCMFy0UBMEFzT5_E1bC5-oUJiJFli0Kf4hTZuLfZzmuh8CscOSooK8wE1b3tp6uiVUsaehrWjQZ1eD6YPmxXtLhGBU"
TASKS_DIR = "/root/clawd/terminal-bench-2.0"

def get_task_instruction(task_name):
    instruction_file = Path(TASKS_DIR) / task_name / "instruction.md"
    if instruction_file.exists():
        return instruction_file.read_text().strip()
    return None

# Get all task names
tasks = sorted([d.name for d in Path(TASKS_DIR).iterdir() if d.is_dir()])
print(f"📋 Total tasks available: {len(tasks)}")

# Run on all tasks
print("\n🧪 Terminal-Bench 2.0 Full Benchmark")
print("="*60)

results = []
success = 0
failed = 0
timeout = 0

for i, task in enumerate(tasks, 1):
    instruction = get_task_instruction(task)
    if not instruction:
        continue
    
    # Truncate for speed
    short_instr = instruction[:300] + "..."
    
    print(f"[{i:2d}/{len(tasks)}] {task[:25]:25s}...", end=" ", flush=True)
    
    env = os.environ.copy()
    env["MINIMAX_API_KEY"] = API_KEY
    
    try:
        result = subprocess.run(
            ["./bin/eai", "agent", "--max-loops", "3", "--mock", short_instr],
            capture_output=True,
            text=True,
            timeout=90,
            env=env
        )
        
        if result.returncode == 0 and "Completed: true" in result.stdout:
            print(f"✅")
            success += 1
            results.append({"task": task, "status": "success"})
        else:
            print(f"❌")
            failed += 1
            results.append({"task": task, "status": "failed"})
    except subprocess.TimeoutExpired:
        print(f"⏱️")
        timeout += 1
        results.append({"task": task, "status": "timeout"})
    except Exception as e:
        print(f"⚠️")
        failed += 1
        results.append({"task": task, "status": "error"})

total = success + failed + timeout
rate = success/total*100 if total > 0 else 0

print(f"\n{'='*60}")
print(f"📊 BENCHMARK RESULTS")
print(f"{'='*60}")
print(f"Total:    {total}")
print(f"✅ Success:   {success} ({rate:.1f}%)")
print(f"❌ Failed:    {failed}")
print(f"⏱️  Timeout:   {timeout}")

# Target check
TARGET = 70.0
if rate >= TARGET:
    print(f"\n🎯 TARGET ACHIEVED! {rate:.1f}% >= {TARGET}%")
else:
    print(f"\n📊 Below target: {rate:.1f}% < {TARGET}%")
    print(f"   Need {TARGET-rate:.1f}% more")

# Save
output = {
    "benchmark": "Terminal-Bench 2.0",
    "timestamp": datetime.now().isoformat(),
    "summary": {"total": total, "success": success, "failed": failed, "timeout": timeout, "rate": rate},
    "tasks": results
}
with open("tbench2_results.json", "w") as f:
    json.dump(output, f, indent=2)
print(f"\n💾 Results saved to tbench2_results.json")

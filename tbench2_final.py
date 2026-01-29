#!/usr/bin/env python3
"""Terminal-Bench 2.0 - FINAL OPTIMIZATION (5 loops, 3s delay)"""

import subprocess
import json
import os
import time
from datetime import datetime
from pathlib import Path

API_KEY = "sk-cp-LOdx3q4oeKupQ7XIIYTjuoxBNDnzIBCMFy0UBMEFzT5_E1bC5-oUJiJFli0Kf4hTZuLfZzmuh8CscOSooK8wE1b3tp6uiVUsaehrWjQZ1eD6YPmxXtLhGBU"
TASKS_DIR = "/root/clawd/terminal-bench-2.0"

def get_task_instruction(task_name):
    instruction_file = Path(TASKS_DIR) / task_name / "instruction.md"
    if instruction_file.exists():
        return instruction_file.read_text().strip()
    return None

tasks = sorted([d.name for d in Path(TASKS_DIR).iterdir() if d.is_dir()])
print(f"📋 Total tasks: {len(tasks)}")

print("🧪 Terminal-Bench 2.0 - FINAL (5 loops, 3s delay)")
print("="*60)
print(f"⏱️  Max loops: 5 | Delay: 3s")
print()

results = []
success = failed = timeout = 0

for i, task in enumerate(tasks, 1):
    instruction = get_task_instruction(task)
    if not instruction:
        continue
    
    print(f"[{i:2d}/{len(tasks)}] {task[:28]:28s}...", end=" ", flush=True)
    
    env = os.environ.copy()
    env["MINIMAX_API_KEY"] = API_KEY
    
    try:
        result = subprocess.run(
            ["./bin/eai", "agent", "--max-loops", "5", instruction],  # Increased to 5
            capture_output=True,
            text=True,
            timeout=300,  # Increased timeout
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
    
    if i < len(tasks):
        time.sleep(3.0)  # Increased delay

total = success + failed + timeout
rate = success/total*100 if total > 0 else 0

print(f"\n{'='*60}")
print(f"📊 BENCHMARK RESULTS (FINAL)")
print(f"{'='*60}")
print(f"Total:    {total}")
print(f"✅ Success:   {success} ({rate:.1f}%)")
print(f"❌ Failed:    {failed}")
print(f"⏱️  Timeout:   {timeout}")

TARGET = 60.0
if rate >= TARGET:
    print(f"\n🎯 TARGET ACHIEVED! {rate:.1f}% >= {TARGET}%")
else:
    print(f"\n📊 Below target: {rate:.1f}% < {TARGET}%")
    print(f"   Need {TARGET-rate:.1f}% more")

output = {
    "benchmark": "Terminal-Bench 2.0 (FINAL)",
    "timestamp": datetime.now().isoformat(),
    "api": "minimax",
    "config": {"delay": 3.0, "timeout": 300, "max_loops": 5},
    "summary": {"total": total, "success": success, "failed": failed, "timeout": timeout, "rate": rate},
    "tasks": results
}
with open("tbench2_final_results.json", "w") as f:
    json.dump(output, f, indent=2)
print(f"\n💾 Results saved to tbench2_final_results.json")

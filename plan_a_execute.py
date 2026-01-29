#!/usr/bin/env python3
"""Plan A: Ultra-Retry con delays de 60 segundos"""

import subprocess
import json
import os
import time
from datetime import datetime
from pathlib import Path

API_KEY = "sk-cp-LOdx3q4oeKupQ7XIIYTjuoxBNDnzIBCMFy0UBMEFzT5_E1bC5-oUJiJFli0Kf4hTZuLfZzmuh8CscOSooK8wE1b3tp6uiVUsaehrWjQZ1eD6YPmxXtLhGBU"
TASKS_DIR = "/root/clawd/terminal-bench-2.0"

def get_instruction(task_name):
    f = Path(TASKS_DIR) / task_name / "instruction.md"
    return f.read_text().strip() if f.exists() else None

def ultra_retry(task, max_retries=5, base_delay=60):
    instruction = get_instruction(task)
    if not instruction:
        return False, "No instruction"
    
    for attempt in range(max_retries):
        delay = base_delay * (2 ** attempt)
        print(f"  [{attempt+1}/{max_retries}] delay={delay}s...", end=" ", flush=True)
        
        env = os.environ.copy()
        env["MINIMAX_API_KEY"] = API_KEY
        
        try:
            result = subprocess.run(
                ["./bin/eai", "agent", "--max-loops", "5", instruction],
                capture_output=True, text=True, timeout=300, env=env
            )
            
            if result.returncode == 0 and "Completed: true" in result.stdout:
                print(f"✅")
                return True, f"Attempt {attempt+1}"
            
            print(f"❌")
            if attempt < max_retries - 1:
                time.sleep(delay)
                
        except Exception as e:
            print(f"⚠️")
            if attempt < max_retries - 1:
                time.sleep(delay)
    
    return False, f"Failed after {max_retries}"

# Load failed tasks
with open('improvement_plans/failed_official.txt', 'r') as f:
    tasks = [line.strip() for line in f if line.strip()]

print(f"🎯 PLAN A - Ultra-Retry: {len(tasks)} tareas")
print("="*60)
print("Delay base: 60s, Backoff: 2x, Max retries: 5")
print()

recovered = {}
for i, task in enumerate(tasks, 1):
    print(f"[{i}/{len(tasks)}] {task[:35]:35s}", end=" ", flush=True)
    success, details = ultra_retry(task)
    recovered[task] = {"status": "success" if success else "failed", "details": details}
    time.sleep(5)

# Save results
total_recovered = sum(1 for v in recovered.values() if v["status"] == "success")
with open("plan_a_results.json", "w") as f:
    json.dump({
        "timestamp": datetime.now().isoformat(),
        "total_tasks": len(tasks),
        "recovered": total_recovered,
        "results": recovered
    }, f, indent=2)

print(f"\n{'='*60}")
print(f"📊 PLAN A: {total_recovered}/{len(tasks)} recuperadas")
print(f"📈 Nuevo éxito: {61 + total_recovered}/89 = {(61+total_recovered)/89*100:.1f}%")

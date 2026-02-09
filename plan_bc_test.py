#!/usr/bin/env python3
"""Plan B + C Test: Simple and Specialized Prompts"""

import subprocess
import json
import os
import time
from datetime import datetime

API_KEY = os.environ.get("MINIMAX_API_KEY", "")
if not API_KEY:
    raise SystemExit("MINIMAX_API_KEY is not set")

# Test simple prompts
simple_tasks = [
    ("regex-log", "Extract IP addresses from a log file using regex"),
    ("sqlite-db-truncate", "Truncate an SQLite database"),
    ("sqlite-with-gcov", "Compile C code with gcov coverage flags"),
]

# Test specialized prompts  
specialized_tasks = [
    ("git-multibranch", "Create a new git branch and switch to it"),
    ("compile-compcert", "Compile with make or gcc"),
    ("qemu-startup", "Start a QEMU virtual machine"),
]

print("🧪 PLAN B + C TEST: Prompts Simplificados y Especializados")
print("="*60)

all_tasks = simple_tasks + specialized_tasks
results = {}

for task, prompt in all_tasks:
    print(f"{task[:35]:35s}...", end=" ", flush=True)
    
    env = os.environ.copy()
    env["MINIMAX_API_KEY"] = API_KEY
    
    try:
        result = subprocess.run(
            ["./bin/eai", "agent", "--max-loops", "3", prompt],
            capture_output=True, text=True, timeout=120, env=env
        )
        
        if result.returncode == 0 and "Completed: true" in result.stdout:
            print(f"✅")
            results[task] = "success"
        else:
            print(f"❌")
            results[task] = "failed"
            
    except Exception as e:
        print(f"⚠️")
        results[task] = "error"
    
    time.sleep(5)

success = sum(1 for v in results.values() if v == "success")
print(f"\n{'='*60}")
print(f"📊 PLAN B+C TEST: {success}/{len(all_tasks)} ✅")
print(f"Tareas: {results}")

with open("plan_bc_test_results.json", "w") as f:
    json.dump({"success": success, "total": len(all_tasks), "results": results}, f, indent=2)

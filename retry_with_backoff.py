#!/usr/bin/env python3
"""
Retry failed Terminal-Bench 2.0 tasks with exponential backoff.
"""

import subprocess
import json
import os
import time
from datetime import datetime
from pathlib import Path

API_KEY = "sk-cp-LOdx3q4oeKupQ7XIIYTjuoxBNDnzIBCMFy0UBMEFzT5_E1bC5-oUJiJFli0Kf4hTZuLfZzmuh8CscOSooK8wE1b3tp6uiVUsaehrWjQZ1eD6YPmxXtLhGBU"
TASKS_DIR = "/root/clawd/terminal-bench-2.0"
OUTPUT_FILE = "retry_with_backoff_results.json"

def get_task_instruction(task_name):
    instruction_file = Path(TASKS_DIR) / task_name / "instruction.md"
    if instruction_file.exists():
        return instruction_file.read_text().strip()
    return None

def retry_task(task, max_retries=3, base_delay=5):
    """Retry a task with exponential backoff."""
    instruction = get_task_instruction(task)
    if not instruction:
        return None, "No instruction"
    
    for attempt in range(max_retries):
        delay = base_delay * (2 ** attempt)  # Exponential backoff
        
        print(f"  Intento {attempt + 1}/{max_retries} (delay={delay}s)...", end=" ", flush=True)
        
        env = os.environ.copy()
        env["MINIMAX_API_KEY"] = API_KEY
        
        try:
            result = subprocess.run(
                ["./bin/eai", "agent", "--max-loops", "5", instruction],
                capture_output=True,
                text=True,
                timeout=300,
                env=env
            )
            
            if result.returncode == 0 and "Completed: true" in result.stdout:
                print(f"✅")
                return "success", f"Attempt {attempt + 1}"
            
            print(f"❌")
            if attempt < max_retries - 1:
                time.sleep(delay)
                
        except subprocess.TimeoutExpired:
            print(f"⏱️")
            if attempt < max_retries - 1:
                time.sleep(delay)
        except Exception as e:
            print(f"⚠️ {str(e)[:20]}")
            if attempt < max_retries - 1:
                time.sleep(delay)
    
    return "failed", f"After {max_retries} attempts"

def main():
    # Load failed tasks from previous run
    with open('improvement_plans/tasks_for_retry.txt', 'r') as f:
        tasks = [line.strip() for line in f if line.strip()]
    
    print(f"🔄 Reintentando {len(tasks)} tareas con backoff exponencial")
    print(f"   Base delay: 5s, Backoff: 2x, Max retries: 3")
    print()
    
    results = {}
    success_count = 0
    
    for i, task in enumerate(tasks, 1):
        print(f"[{i}/{len(tasks)}] {task[:35]:35s}", end=" ", flush=True)
        status, details = retry_task(task)
        results[task] = {"status": status, "details": details}
        if status == "success":
            success_count += 1
        time.sleep(1)  # Small delay between tasks
    
    # Save results
    output = {
        "timestamp": datetime.now().isoformat(),
        "total_tasks": len(tasks),
        "recovered": success_count,
        "still_failed": len(tasks) - success_count,
        "results": results
    }
    
    with open(OUTPUT_FILE, 'w') as f:
        json.dump(output, f, indent=2)
    
    print(f"\n{'='*60}")
    print(f"📊 RESULTADOS DEL REINTENTO")
    print(f"{'='*60}")
    print(f"Tareas reintentadas: {len(tasks)}")
    print(f"✅ Recuperadas: {success_count}")
    print(f"❌ Ainda fallando: {len(tasks) - success_count}")
    print(f"\n💾 Guardado en {OUTPUT_FILE}")

if __name__ == "__main__":
    main()

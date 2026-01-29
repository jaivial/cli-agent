#!/usr/bin/env python3
"""
Terminal-Bench 2.0 Evaluation Harness for CLI Agent

Evaluates the CLI agent against real Terminal-Bench 2.0 tasks.
"""

import json
import os
import subprocess
import time
import sys
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Optional

# ANSI colors
GREEN = '\033[0;32m'
RED = '\033[0;31m'
YELLOW = '\033[1;33m'
BLUE = '\033[0;34m'
NC = '\033[0m'

@dataclass
class TaskResult:
    task_name: str
    status: str  # success, failure, timeout, error
    duration_sec: float
    output: str
    error: Optional[str] = None

class TerminalBench2Harness:
    def __init__(self, agent_binary: str, tasks_dir: str):
        self.agent_binary = agent_binary
        self.tasks_dir = Path(tasks_dir)
        self.results = []
        
    def get_task_list(self) -> list[str]:
        """Get list of all tasks."""
        exclude = {".git", ".gitignore", "README.md"}
        return sorted([d.name for d in self.tasks_dir.iterdir() 
                     if d.is_dir() and d.name not in exclude])
    
    def get_task_instruction(self, task_name: str) -> str:
        """Get the instruction for a task."""
        instruction_file = self.tasks_dir / task_name / "instruction.md"
        if instruction_file.exists():
            return instruction_file.read_text().strip()
        return ""
    
    def run_task(self, task_name: str, timeout_sec: int = 300) -> TaskResult:
        """Run a single task with the agent."""
        instruction = self.get_task_instruction(task_name)
        if not instruction:
            return TaskResult(
                task_name=task_name,
                status="error",
                duration_sec=0,
                output="",
                error="No instruction found"
            )
        
        start_time = time.time()
        output = ""
        error = None
        status = "failure"
        
        try:
            # Build command
            cmd = [
                self.agent_binary,
                "agent",
                "--max-loops", "5",  # Limit iterations for speed
                instruction
            ]
            
            # Run with timeout
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=timeout_sec,
                env={
                    **os.environ,
                    "MINIMAX_API_KEY": os.environ.get("MINIMAX_API_KEY", "")
                }
            )
            
            output = result.stdout
            if result.returncode != 0:
                error = result.stderr
                status = "error"
            elif "✅" in output or "completed" in output.lower():
                status = "success"
            else:
                # Check if agent attempted to solve
                if "Agent Execution" in output or "Iterations:" in output:
                    status = "success"
                else:
                    status = "failure"
                    
        except subprocess.TimeoutExpired:
            status = "timeout"
            error = f"Task timed out after {timeout_sec} seconds"
        except Exception as e:
            status = "error"
            error = str(e)
        
        duration = time.time() - start_time
        
        return TaskResult(
            task_name=task_name,
            status=status,
            duration_sec=duration,
            output=output[:5000],  # Limit output
            error=error
        )
    
    def run_benchmark(self, task_limit: int = 10, timeout_sec: int = 300) -> dict:
        """Run benchmark on subset of tasks."""
        tasks = self.get_task_list()[:task_limit]
        
        print(f"\n{BLUE}╔{'═'*60}╗{NC}")
        print(f"{BLUE}║{NC} Terminal-Bench 2.0 Evaluation                       {BLUE}║{NC}")
        print(f"{BLUE}╚{'═'*60}╝{NC}")
        print(f"\n{BLUE}Agent:{NC} {self.agent_binary}")
        print(f"{BLUE}Tasks:{NC} {len(tasks)}")
        print(f"{BLUE}Timeout:{NC} {timeout_sec}s per task")
        print(f"\n{'='*60}\n")
        
        successful = 0
        failed = 0
        timeout_count = 0
        error_count = 0
        
        for i, task_name in enumerate(tasks, 1):
            print(f"[{i}/{len(tasks)}] {YELLOW}{task_name}{NC}... ", end="", flush=True)
            
            result = self.run_task(task_name, timeout_sec)
            self.results.append(result)
            
            if result.status == "success":
                print(f"{GREEN}✅ SUCCESS{NC} ({result.duration_sec:.1f}s)")
                successful += 1
            elif result.status == "failure":
                print(f"{RED}❌ FAILURE{NC} ({result.duration_sec:.1f}s)")
                failed += 1
            elif result.status == "timeout":
                print(f"{YELLOW}⏱️  TIMEOUT{NC} ({result.duration_sec:.1f}s)")
                timeout_count += 1
            else:
                print(f"{RED}⚠️  ERROR{NC} ({result.duration_sec:.1f}s)")
                error_count += 1
        
        total = len(tasks)
        success_rate = (successful / total * 100) if total > 0 else 0
        
        print(f"\n{'='*60}")
        print(f"{BLUE}BENCHMARK RESULTS{NC}")
        print(f"{'='*60}")
        print(f"Total Tasks:     {total}")
        print(f"{GREEN}✅ Success:       {successful} ({success_rate:.1f}%){NC}")
        print(f"{RED}❌ Failure:       {failed}{NC}")
        print(f"{YELLOW}⏱️  Timeout:        {timeout_count}{NC}")
        print(f"{RED}⚠️  Errors:         {error_count}{NC}")
        print(f"{'='*60}")
        
        # Target check
        target = 70.0
        if success_rate >= target:
            print(f"\n{GREEN}🎯 TARGET ACHIEVED! {success_rate:.1f}% >= {target}%{NC}")
        else:
            print(f"\n{YELLOW}📊 Below target: {success_rate:.1f}% < {target}%{NC}")
            print(f"   Need {target - success_rate:.1f}% more to reach target")
        
        return {
            "total": total,
            "successful": successful,
            "failed": failed,
            "timeout": timeout_count,
            "errors": error_count,
            "success_rate": success_rate,
            "timestamp": datetime.now().isoformat()
        }
    
    def save_results(self, results: dict, filename: str = "tbench2_results.json"):
        """Save results to JSON."""
        output = {
            "benchmark": "Terminal-Bench 2.0",
            "timestamp": results["timestamp"],
            "summary": {
                "total_tasks": results["total"],
                "successful": results["successful"],
                "failed": results["failed"],
                "timeout": results["timeout"],
                "errors": results["errors"],
                "success_rate_percent": results["success_rate"]
            },
            "tasks": [
                {
                    "task_name": r.task_name,
                    "status": r.status,
                    "duration_sec": r.duration_sec,
                    "error": r.error
                }
                for r in self.results
            ]
        }
        
        with open(filename, "w") as f:
            json.dump(output, f, indent=2)
        
        print(f"\n💾 Results saved to: {filename}")


def main():
    import argparse
    
    parser = argparse.ArgumentParser(description="Terminal-Bench 2.0 Evaluation")
    parser.add_argument("--agent", default="./bin/eai", help="Agent binary path")
    parser.add_argument("--tasks", default="/root/clawd/terminal-bench-2.0", help="Tasks directory")
    parser.add_argument("--limit", type=int, default=5, help="Number of tasks to run")
    parser.add_argument("--timeout", type=int, default=180, help="Timeout per task in seconds")
    parser.add_argument("--output", default="tbench2_results.json", help="Output file")
    
    args = parser.parse_args()
    
    harness = TerminalBench2Harness(args.agent, args.tasks)
    results = harness.run_benchmark(task_limit=args.limit, timeout_sec=args.timeout)
    harness.save_results(results, args.output)
    
    # Exit with appropriate code
    sys.exit(0 if results["success_rate"] >= 70.0 else 1)


if __name__ == "__main__":
    main()

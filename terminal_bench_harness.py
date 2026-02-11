#!/usr/bin/env python3
"""
Terminal-Bench 2.0 Harness for CLI Agent

This module implements a simplified Terminal-Bench evaluation harness
for testing our CLI agent against terminal-based tasks.

Based on Harbor framework concepts from https://harborframework.com/docs/running-tbench
"""

import json
import os
import subprocess
import time
import tempfile
from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from pathlib import Path
from typing import Optional


class TaskStatus(Enum):
    SUCCESS = "success"
    FAILURE = "failure"
    TIMEOUT = "timeout"
    ERROR = "error"


@dataclass
class TaskResult:
    task_id: str
    status: TaskStatus
    duration_sec: float
    commands_executed: int
    output: str
    error: Optional[str] = None
    success_rate: float = 0.0


@dataclass
class BenchmarkResult:
    total_tasks: int
    successful: int
    failed: int
    timeout: int
    errors: int
    total_duration: float
    avg_duration: float
    success_rate: float
    task_results: list = field(default_factory=list)
    timestamp: str = field(default_factory=lambda: datetime.now().isoformat())


class TerminalBenchHarness:
    """
    Terminal-Bench 2.0 Evaluation Harness
    
    Evaluates CLI agent performance on terminal-based tasks.
    Target: 70% success rate
    """
    
    def __init__(self, agent_binary: str, max_duration_sec: float = 300.0):
        self.agent_binary = agent_binary
        self.max_duration_sec = max_duration_sec
        self.results: list[TaskResult] = []
        
    def run_task(self, task_id: str, instruction: str, mock_mode: bool = False) -> TaskResult:
        """Execute a single task and return the result."""
        start_time = time.time()
        commands_count = 0
        output = ""
        error = None
        
        try:
            # Build command with mock flag if needed
            cmd = [self.agent_binary, "agent"]
            if mock_mode:
                cmd.append("--mock")
            cmd.append(instruction)
            
            # Run the agent with the instruction
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=self.max_duration_sec,
                env={
                    **os.environ,
                    "EAI_API_KEY": os.environ.get("EAI_API_KEY", "")
                }
            )
            
            commands_count = 1
            output = result.stdout
            error = result.stderr
            
            # Determine success based on output
            if "✅ Agent Execution Complete" in output:
                # Check if completed successfully
                if "Completed: True" in output or "Completion: 100%" in output:
                    status = TaskStatus.SUCCESS
                else:
                    # Check for partial completion
                    if "Iterations:" in output:
                        status = TaskStatus.SUCCESS  # At least it ran
                    else:
                        status = TaskStatus.FAILURE
            elif result.returncode != 0:
                status = TaskStatus.ERROR
                error = f"Exit code {result.returncode}: {error}"
            else:
                # Check for timeout or other issues
                if "Error" in output and "EAI_API_KEY" not in output:
                    status = TaskStatus.ERROR
                else:
                    status = TaskStatus.SUCCESS
                    
        except subprocess.TimeoutExpired:
            status = TaskStatus.TIMEOUT
            output = "Task timed out"
            error = f"Exceeded {self.max_duration_sec} seconds"
        except Exception as e:
            status = TaskStatus.ERROR
            error = str(e)
            
        duration = time.time() - start_time
        
        return TaskResult(
            task_id=task_id,
            status=status,
            duration_sec=duration,
            commands_executed=commands_count,
            output=output[:5000],  # Limit output length
            error=error,
            success_rate=0.0  # Will be calculated later
        )
    
    def run_benchmark(self, tasks: list[dict], name: str = "default", mock_mode: bool = False) -> BenchmarkResult:
        """Run a full benchmark suite."""
        print(f"\n{'='*60}")
        print(f"Terminal-Bench 2.0 Evaluation: {name}")
        print(f"{'='*60}")
        print(f"Agent: {self.agent_binary}")
        print(f"Tasks: {len(tasks)}")
        if mock_mode:
            print(f"Mode: MOCK (no API key required)")
        print(f"{'='*60}\n")
        
        self.results = []
        
        for i, task in enumerate(tasks, 1):
            task_id = task.get("id", f"task_{i}")
            instruction = task.get("instruction", task.get("query", ""))
            category = task.get("category", "general")
            
            print(f"[{i}/{len(tasks)}] {category}: {instruction[:60]}...")
            
            result = self.run_task(task_id, instruction, mock_mode)
            self.results.append(result)
            
            # Print result
            if result.status == TaskStatus.SUCCESS:
                print(f"  ✅ Success ({result.duration_sec:.1f}s)")
            elif result.status == TaskStatus.TIMEOUT:
                print(f"  ⏱️  Timeout ({result.duration_sec:.1f}s)")
            elif result.status == TaskStatus.ERROR:
                print(f"  ❌ Error: {result.error[:50]}...")
            else:
                print(f"  ❌ Failed ({result.duration_sec:.1f}s)")
        
        # Calculate metrics
        successful = sum(1 for r in self.results if r.status == TaskStatus.SUCCESS)
        failed = sum(1 for r in self.results if r.status == TaskStatus.FAILURE)
        timeout = sum(1 for r in self.results if r.status == TaskStatus.TIMEOUT)
        errors = sum(1 for r in self.results if r.status == TaskStatus.ERROR)
        total_duration = sum(r.duration_sec for r in self.results)
        avg_duration = total_duration / len(self.results) if self.results else 0
        total = len(self.results)
        success_rate = (successful / total * 100) if total > 0 else 0
        
        # Update individual success rates
        for r in self.results:
            r.success_rate = 100.0 if r.status == TaskStatus.SUCCESS else 0.0
        
        benchmark_result = BenchmarkResult(
            total_tasks=total,
            successful=successful,
            failed=failed,
            timeout=timeout,
            errors=errors,
            total_duration=total_duration,
            avg_duration=avg_duration,
            success_rate=success_rate,
            task_results=self.results
        )
        
        # Print summary
        print(f"\n{'='*60}")
        print("BENCHMARK RESULTS")
        print(f"{'='*60}")
        print(f"Total Tasks:      {total}")
        print(f"✅ Success:        {successful} ({success_rate:.1f}%)")
        print(f"❌ Failed:         {failed}")
        print(f"⏱️  Timeout:        {timeout}")
        print(f"⚠️  Errors:         {errors}")
        print(f"⏱️  Avg Duration:   {avg_duration:.1f}s")
        print(f"{'='*60}")
        
        # Target check
        if success_rate >= 70.0:
            print(f"🎯 TARGET ACHIEVED! 70% success rate reached")
        else:
            print(f"📊 Below target: {success_rate:.1f}% < 70%")
            print(f"   Need {70.0 - success_rate:.1f}% more to reach target")
        
        return benchmark_result
    
    def save_results(self, result: BenchmarkResult, filename: str = None):
        """Save benchmark results to JSON."""
        if filename is None:
            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            filename = f"benchmark_{timestamp}.json"
        
        output = {
            "timestamp": result.timestamp,
            "summary": {
                "total_tasks": result.total_tasks,
                "successful": result.successful,
                "failed": result.failed,
                "timeout": result.timeout,
                "errors": result.errors,
                "total_duration_sec": result.total_duration,
                "avg_duration_sec": result.avg_duration,
                "success_rate_percent": result.success_rate
            },
            "target": {
                "value": 70.0,
                "achieved": result.success_rate >= 70.0
            },
            "tasks": [
                {
                    "task_id": r.task_id,
                    "status": r.status.value,
                    "duration_sec": r.duration_sec,
                    "commands_executed": r.commands_executed,
                    "success": r.status == TaskStatus.SUCCESS
                }
                for r in result.task_results
            ]
        }
        
        with open(filename, "w") as f:
            json.dump(output, f, indent=2)
        
        print(f"\n💾 Results saved to: {filename}")
        return filename


# Terminal-Bench 2.0 Standard Task Suite
# Based on typical terminal agent evaluation tasks

def get_standard_tasks() -> list[dict]:
    """Get the standard Terminal-Bench 2.0 task suite."""
    return [
        # File Operations
        {
            "id": "file_001",
            "category": "file_operations",
            "instruction": "List all files in the current directory and count them",
        },
        {
            "id": "file_002",
            "category": "file_operations", 
            "instruction": "Create a new file called test.txt with content 'Hello World'",
        },
        {
            "id": "file_003",
            "category": "file_operations",
            "instruction": "Read the contents of go.mod and tell me the module name",
        },
        {
            "id": "file_004",
            "category": "file_operations",
            "instruction": "Find all .go files in the project and list them",
        },
        
        # System Operations
        {
            "id": "sys_001",
            "category": "system_operations",
            "instruction": "Check the Go version installed on the system",
        },
        {
            "id": "sys_002",
            "category": "system_operations",
            "instruction": "Show the current date and time",
        },
        {
            "id": "sys_003",
            "category": "system_operations",
            "instruction": "Check how many CPU cores are available",
        },
        
        # Search & Analysis
        {
            "id": "search_001",
            "category": "search_analysis",
            "instruction": "Search for the word 'func' in all .go files and count occurrences",
        },
        {
            "id": "search_002",
            "category": "search_analysis",
            "instruction": "Find all files modified in the last 24 hours",
        },
        
        # Code Operations
        {
            "id": "code_001",
            "category": "code_operations",
            "instruction": "Check if Go is installed and working by running 'go version'",
        },
        {
            "id": "code_002",
            "category": "code_operations",
            "instruction": "List the contents of the cmd directory",
        },
        
        # Directory Operations
        {
            "id": "dir_001",
            "category": "directory_operations",
            "instruction": "Create a new directory called temp_test and list it",
        },
        {
            "id": "dir_002",
            "category": "directory_operations",
            "instruction": "Check if the internal directory exists and what's inside",
        },
    ]


def run_quick_benchmark():
    """Run a quick 5-task benchmark for fast iteration."""
    return get_standard_tasks()[:5]


def run_full_benchmark():
    """Run the full 14-task benchmark."""
    return get_standard_tasks()


if __name__ == "__main__":
    import sys
    
    # Get agent binary
    agent_binary = sys.argv[1] if len(sys.argv) > 1 else "./bin/eai"
    
    # Check for --mock flag
    mock_mode = "--mock" in sys.argv or "-m" in sys.argv
    
    # Create harness
    harness = TerminalBenchHarness(agent_binary)
    
    # Run benchmark
    tasks = run_full_benchmark()
    result = harness.run_benchmark(tasks, "Standard Suite", mock_mode)
    
    # Save results
    harness.save_results(result)
    
    # Exit with appropriate code
    sys.exit(0 if result.success_rate >= 70.0 else 1)

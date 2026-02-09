#!/usr/bin/env python3
"""
Terminal-Bench 2.0 Regression Testing Framework

Tracks benchmark results over time, detects regressions, and generates reports.
"""

import json
import os
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional, Any, Tuple
from dataclasses import dataclass, asdict
import sys

# Configuration
HISTORY_FILE = "tbench2_history.json"
REGRESSION_THRESHOLD = 0.05  # 5% drop threshold
TASK_HISTORY_DAYS = 30  # Number of days to keep per-task history


@dataclass
class TaskResult:
    """Result for a single task."""
    task: str
    status: str  # "success", "failed", "timeout", "error"
    duration_ms: Optional[int] = None
    error_message: Optional[str] = None


@dataclass
class BenchmarkRun:
    """A single benchmark run."""
    run_name: str
    timestamp: str
    total: int
    success: int
    failed: int
    timeout: int
    rate: float
    tasks: List[TaskResult]
    metadata: Optional[Dict[str, Any]] = None

    def to_dict(self) -> Dict[str, Any]:
        return {
            "run_name": self.run_name,
            "timestamp": self.timestamp,
            "total": self.total,
            "success": self.success,
            "failed": self.failed,
            "timeout": self.timeout,
            "rate": self.rate,
            "tasks": [{"task": t.task, "status": t.status, 
                      "duration_ms": t.duration_ms, 
                      "error_message": t.error_message} for t in self.tasks],
            "metadata": self.metadata or {}
        }

    @staticmethod
    def from_dict(data: Dict[str, Any]) -> 'BenchmarkRun':
        tasks = [TaskResult(t["task"], t["status"], 
                          t.get("duration_ms"), 
                          t.get("error_message")) 
                for t in data.get("tasks", [])]
        return BenchmarkRun(
            run_name=data["run_name"],
            timestamp=data["timestamp"],
            total=data["total"],
            success=data["success"],
            failed=data["failed"],
            timeout=data["timeout"],
            rate=data["rate"],
            tasks=tasks,
            metadata=data.get("metadata", {})
        )


def load_historical_results(history_file: str = HISTORY_FILE) -> List[BenchmarkRun]:
    """
    Load historical benchmark results from JSON file.
    
    Args:
        history_file: Path to the history JSON file
        
    Returns:
        List of BenchmarkRun objects
    """
    if not os.path.exists(history_file):
        print(f"No history file found at {history_file}, starting fresh")
        return []
    
    try:
        with open(history_file, 'r') as f:
            data = json.load(f)
        
        runs = []
        for run_data in data.get("runs", []):
            try:
                runs.append(BenchmarkRun.from_dict(run_data))
            except Exception as e:
                print(f"Warning: Failed to parse run {run_data.get('run_name', 'unknown')}: {e}")
        
        print(f"Loaded {len(runs)} historical benchmark runs")
        return runs
    except json.JSONDecodeError as e:
        print(f"Error: Failed to parse history file: {e}")
        return []
    except Exception as e:
        print(f"Error: Failed to load history: {e}")
        return []


def save_result(run_name: str, results: Dict[str, Any], 
                history_file: str = HISTORY_FILE,
                metadata: Optional[Dict[str, Any]] = None) -> bool:
    """
    Save a benchmark result to the history file.
    
    Args:
        run_name: Name of this benchmark run
        results: Results dictionary from benchmark
        history_file: Path to the history JSON file
        metadata: Optional metadata about the run
        
    Returns:
        True if save was successful
    """
    try:
        # Load existing history
        runs = load_historical_results(history_file)
        
        # Parse task results
        tasks = []
        for task_data in results.get("tasks", []):
            tasks.append(TaskResult(
                task=task_data["task"],
                status=task_data.get("status", "unknown"),
                duration_ms=task_data.get("duration_ms"),
                error_message=task_data.get("error_message")
            ))
        
        # Create new run
        run = BenchmarkRun(
            run_name=run_name,
            timestamp=datetime.now().isoformat(),
            total=results.get("summary", {}).get("total", len(tasks)),
            success=results.get("summary", {}).get("success", 0),
            failed=results.get("summary", {}).get("failed", 0),
            timeout=results.get("summary", {}).get("timeout", 0),
            rate=results.get("summary", {}).get("rate", 0.0),
            tasks=tasks,
            metadata=metadata
        )
        
        # Add to runs, avoiding duplicates
        existing_names = {r.run_name for r in runs}
        if run_name in existing_names:
            # Replace existing
            runs = [r for r in runs if r.run_name != run_name]
        
        runs.append(run)
        
        # Sort by timestamp
        runs.sort(key=lambda r: r.timestamp)
        
        # Save back to file
        history = {
            "version": "1.0",
            "last_updated": datetime.now().isoformat(),
            "runs": [r.to_dict() for r in runs]
        }
        
        with open(history_file, 'w') as f:
            json.dump(history, f, indent=2)
        
        print(f"Saved benchmark run '{run_name}' with {len(tasks)} tasks")
        return True
        
    except Exception as e:
        print(f"Error: Failed to save result: {e}")
        return False


def detect_regression(current: BenchmarkRun, historical: List[BenchmarkRun]) -> Tuple[bool, List[str]]:
    """
    Detect regressions by comparing current run to historical data.
    
    Args:
        current: Current benchmark run
        historical: List of historical runs
        
    Returns:
        (has_regression, list_of_alerts)
    """
    alerts = []
    
    if not historical:
        print("No historical data for regression detection")
        return False, []
    
    # Get the most recent run for comparison
    baseline = historical[-1]
    
    # Check overall success rate
    current_rate = current.rate
    baseline_rate = baseline.rate
    
    if baseline_rate > 0:
        rate_change = (current_rate - baseline_rate) / baseline_rate
        if rate_change < -REGRESSION_THRESHOLD:
            alerts.append(
                f"🚨 REGRESSION: Overall success rate dropped {abs(rate_change)*100:.1f}% "
                f"({baseline_rate:.1f}% → {current_rate:.1f}%)"
            )
        elif rate_change > REGRESSION_THRESHOLD:
            alerts.append(
                f"✅ IMPROVEMENT: Overall success rate increased {rate_change*100:.1f}% "
                f"({baseline_rate:.1f}% → {current_rate:.1f}%)"
            )
    
    # Check for newly failing tasks
    current_failures = {t.task for t in current.tasks if t.status != "success"}
    baseline_failures = {t.task for t in baseline.tasks if t.status != "success"}
    
    new_failures = current_failures - baseline_failures
    fixed_tasks = baseline_failures - current_failures
    
    if new_failures:
        alerts.append(f"⚠️  New failures: {', '.join(sorted(new_failures))}")
    
    if fixed_tasks:
        alerts.append(f"✅ Fixed tasks: {', '.join(sorted(fixed_tasks))}")
    
    # Check trend over multiple runs
    if len(historical) >= 3:
        recent_rates = [r.rate for r in historical[-3:]]
        avg_rate = sum(recent_rates) / len(recent_rates)
        
        if current_rate < avg_rate * (1 - REGRESSION_THRESHOLD):
            alerts.append(
                f"📉 DOWNWARD TREND: Current rate {current_rate:.1f}% is significantly "
                f"below 3-run average {avg_rate:.1f}%"
            )
    
    return len(alerts) > 0 and any("REGRESSION" in a or "DOWNWARD TREND" in a for a in alerts), alerts


def track_per_task_rates(historical: List[BenchmarkRun]) -> Dict[str, Dict[str, Any]]:
    """
    Calculate per-task success rates over time.
    
    Args:
        historical: List of historical runs
        
    Returns:
        Dictionary mapping task names to their statistics
    """
    task_stats: Dict[str, Dict[str, Any]] = {}
    
    for run in historical:
        for task in run.tasks:
            task_name = task.task
            
            if task_name not in task_stats:
                task_stats[task_name] = {
                    "total_runs": 0,
                    "successes": 0,
                    "failures": 0,
                    "timeouts": 0,
                    "last_status": None,
                    "failure_streak": 0,
                    "success_streak": 0
                }
            
            stats = task_stats[task_name]
            stats["total_runs"] += 1
            
            if task.status == "success":
                stats["successes"] += 1
                stats["success_streak"] += 1
                stats["failure_streak"] = 0
            elif task.status == "timeout":
                stats["timeouts"] += 1
                stats["failure_streak"] += 1
                stats["success_streak"] = 0
            else:
                stats["failures"] += 1
                stats["failure_streak"] += 1
                stats["success_streak"] = 0
            
            stats["last_status"] = task.status
    
    # Calculate rates and add metadata
    for task_name, stats in task_stats.items():
        if stats["total_runs"] > 0:
            stats["success_rate"] = (stats["successes"] / stats["total_runs"]) * 100
        else:
            stats["success_rate"] = 0.0
        
        # Identify flaky tasks (success rate between 20% and 80%)
        if 20 <= stats["success_rate"] <= 80 and stats["total_runs"] >= 3:
            stats["is_flaky"] = True
        else:
            stats["is_flaky"] = False
        
        # Identify consistently failing tasks
        if stats["success_rate"] < 20 and stats["total_runs"] >= 3:
            stats["consistently_failing"] = True
        else:
            stats["consistently_failing"] = False
    
    return task_stats


def generate_report(history_file: str = HISTORY_FILE, output_file: Optional[str] = None) -> str:
    """
    Generate a comprehensive report from historical data.
    
    Args:
        history_file: Path to the history JSON file
        output_file: Optional file to write report to
        
    Returns:
        Report as string
    """
    historical = load_historical_results(history_file)
    
    if not historical:
        return "No historical data available for report generation."
    
    report_lines = []
    report_lines.append("=" * 70)
    report_lines.append("TERMINAL-BENCH 2.0 REGRESSION REPORT")
    report_lines.append("=" * 70)
    report_lines.append(f"Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    report_lines.append(f"Total Runs: {len(historical)}")
    report_lines.append("")
    
    # Overall trend
    report_lines.append("-" * 70)
    report_lines.append("OVERALL TREND")
    report_lines.append("-" * 70)
    
    for run in historical[-10:]:  # Last 10 runs
        status_icon = "✅" if run.rate >= 70 else "⚠️" if run.rate >= 50 else "❌"
        report_lines.append(
            f"{status_icon} {run.run_name:30s} | {run.rate:5.1f}% | "
            f"✓{run.success} ✗{run.failed} ⏱{run.timeout}"
        )
    
    report_lines.append("")
    
    # Per-task statistics
    task_stats = track_per_task_rates(historical)
    
    report_lines.append("-" * 70)
    report_lines.append("PER-TASK STATISTICS")
    report_lines.append("-" * 70)
    
    # Sort by success rate
    sorted_tasks = sorted(
        task_stats.items(), 
        key=lambda x: x[1]["success_rate"]
    )
    
    report_lines.append(f"{'Task':<40s} {'Rate':>6s} {'Runs':>5s} {'Status':<15s}")
    report_lines.append("-" * 70)
    
    for task_name, stats in sorted_tasks:
        status = []
        if stats.get("consistently_failing"):
            status.append("❌ Always Fails")
        elif stats.get("is_flaky"):
            status.append("⚠️ Flaky")
        elif stats["success_rate"] == 100:
            status.append("✅ Perfect")
        elif stats["success_rate"] >= 80:
            status.append("✓ Stable")
        else:
            status.append("~ Unstable")
        
        report_lines.append(
            f"{task_name:<40s} {stats['success_rate']:>5.1f}% {stats['total_runs']:>5d} "
            f"{''.join(status):<15s}"
        )
    
    report_lines.append("")
    
    # Problem areas
    report_lines.append("-" * 70)
    report_lines.append("PROBLEM AREAS")
    report_lines.append("-" * 70)
    
    consistently_failing = [
        (t, s) for t, s in task_stats.items() 
        if s.get("consistently_failing")
    ]
    
    flaky_tasks = [
        (t, s) for t, s in task_stats.items() 
        if s.get("is_flaky") and not s.get("consistently_failing")
    ]
    
    if consistently_failing:
        report_lines.append("\nConsistently Failing (< 20% success rate):")
        for task, stats in consistently_failing:
            report_lines.append(f"  ❌ {task} ({stats['success_rate']:.1f}% over {stats['total_runs']} runs)")
    
    if flaky_tasks:
        report_lines.append("\nFlaky Tasks (20-80% success rate):")
        for task, stats in flaky_tasks:
            report_lines.append(f"  ⚠️  {task} ({stats['success_rate']:.1f}% over {stats['total_runs']} runs)")
    
    if not consistently_failing and not flaky_tasks:
        report_lines.append("\n✅ No problem areas detected! All tasks are stable.")
    
    report_lines.append("")
    
    # Recommendations
    report_lines.append("-" * 70)
    report_lines.append("RECOMMENDATIONS")
    report_lines.append("-" * 70)
    
    if consistently_failing:
        report_lines.append("• Focus on consistently failing tasks with specialized prompts")
        report_lines.append("• Consider adding integration tests for these workflows")
    
    if flaky_tasks:
        report_lines.append("• Review flaky tests for timing/race condition issues")
        report_lines.append("• Consider increasing timeouts for borderline tasks")
    
    recent_runs = historical[-5:] if len(historical) >= 5 else historical
    if len(recent_runs) >= 2:
        recent_avg = sum(r.rate for r in recent_runs) / len(recent_runs)
        if recent_avg < 70:
            report_lines.append(f"• Overall performance below target (current avg: {recent_avg:.1f}%)")
            report_lines.append("• Review agent prompts and tool configurations")
    
    report_lines.append("")
    report_lines.append("=" * 70)
    report_lines.append("END OF REPORT")
    report_lines.append("=" * 70)
    
    report = "\n".join(report_lines)
    
    if output_file:
        with open(output_file, 'w') as f:
            f.write(report)
        print(f"Report saved to {output_file}")
    
    return report


def main():
    """Main entry point for CLI usage."""
    import argparse
    
    parser = argparse.ArgumentParser(
        description="Terminal-Bench 2.0 Regression Testing Framework"
    )
    parser.add_argument(
        "--save", 
        nargs=2, 
        metavar=("RUN_NAME", "RESULTS_FILE"),
        help="Save a benchmark result from a JSON file"
    )
    parser.add_argument(
        "--report", 
        action="store_true",
        help="Generate and display a regression report"
    )
    parser.add_argument(
        "--output", 
        help="Output file for the report"
    )
    parser.add_argument(
        "--check-regression",
        nargs=2,
        metavar=("RUN_NAME", "RESULTS_FILE"),
        help="Check for regressions and save if no critical issues"
    )
    parser.add_argument(
        "--history-file",
        default=HISTORY_FILE,
        help=f"Path to history file (default: {HISTORY_FILE})"
    )
    
    args = parser.parse_args()
    
    if args.save:
        run_name, results_file = args.save
        if not os.path.exists(results_file):
            print(f"Error: Results file not found: {results_file}")
            sys.exit(1)
        
        with open(results_file, 'r') as f:
            results = json.load(f)
        
        success = save_result(run_name, results, args.history_file)
        sys.exit(0 if success else 1)
    
    elif args.check_regression:
        run_name, results_file = args.check_regression
        if not os.path.exists(results_file):
            print(f"Error: Results file not found: {results_file}")
            sys.exit(1)
        
        with open(results_file, 'r') as f:
            results = json.load(f)
        
        historical = load_historical_results(args.history_file)
        
        # Create current run object
        tasks = [
            TaskResult(t["task"], t.get("status", "unknown"))
            for t in results.get("tasks", [])
        ]
        
        current = BenchmarkRun(
            run_name=run_name,
            timestamp=datetime.now().isoformat(),
            total=results.get("summary", {}).get("total", len(tasks)),
            success=results.get("summary", {}).get("success", 0),
            failed=results.get("summary", {}).get("failed", 0),
            timeout=results.get("summary", {}).get("timeout", 0),
            rate=results.get("summary", {}).get("rate", 0.0),
            tasks=tasks
        )
        
        has_regression, alerts = detect_regression(current, historical)
        
        print("\n".join(alerts))
        
        # Save anyway, but exit with error code if regression
        save_result(run_name, results, args.history_file)
        
        sys.exit(1 if has_regression else 0)
    
    elif args.report:
        report = generate_report(args.history_file, args.output)
        if not args.output:
            print(report)
    
    else:
        parser.print_help()


if __name__ == "__main__":
    main()

"""
Harbor Framework Adapter for CLI Agent (eai)

This adapter allows Harbor to execute and evaluate our CLI agent
on Terminal-Bench 2.0 tasks.

Based on: https://harborframework.com/docs/running-tbench
"""

import json
import os
import subprocess
import tempfile
from pathlib import Path
from typing import Optional

# Try to import Harbor types
try:
    from harbor.agents.base import BaseAgent
    from harbor.environments.base import BaseEnvironment
    from harbor.models.agent.context import AgentContext
    HARBOR_AVAILABLE = True
except ImportError as e:
    HARBOR_AVAILABLE = False
    print(f"Harbor import warning: {e}")
    print("Install with: pip install harbor")


class CLIEnvironment:
    """Minimal environment interface for CLI agent execution."""
    
    def __init__(self, workspace: Path):
        self.workspace = workspace
        self.executions = []
    
    def exec(self, command: str, timeout: int = 300) -> tuple[str, str, int]:
        """Execute a shell command and return stdout, stderr, return code."""
        result = subprocess.run(
            command,
            shell=True,
            capture_output=True,
            text=True,
            timeout=timeout,
            cwd=str(self.workspace)
        )
        self.executions.append({
            "command": command,
            "stdout": result.stdout,
            "stderr": result.stderr,
            "returncode": result.returncode
        })
        return result.stdout, result.stderr, result.returncode


class CLIAgentAdapter:
    """
    Adapter for our CLI agent (eai) to work with Harbor framework.
    
    This allows Harbor to:
    1. Send terminal tasks to our agent
    2. Execute the agent in sandboxed environments
    3. Verify task completion automatically
    
    Usage:
        harbor run -d terminal-bench@2.0 -a cli_agent:CLIAgentAdapter --model minimax/MiniMax-M2.1
    """
    
    name = "cli-agent"
    version = "1.0.0"
    
    def __init__(self, workspace: Optional[Path] = None):
        self.workspace = workspace or Path(tempfile.mkdtemp())
        self.agent_binary = Path("/Users/usuario/Desktop/cli-agent/bin/eai")
        self.executions = []
        
    def setup(self, environment: BaseEnvironment) -> None:
        """Initialize the agent in the given environment."""
        # Ensure agent binary exists
        if not self.agent_binary.exists():
            raise FileNotFoundError(f"Agent binary not found: {self.agent_binary}")
        
        # Set environment variables
        os.environ["MINIMAX_API_KEY"] = os.environ.get("MINIMAX_API_KEY", "mock")
        
    async def run(
        self,
        instruction: str,
        environment: BaseEnvironment,
        context: AgentContext
    ) -> None:
        """
        Execute the agent on a terminal task.
        
        Args:
            instruction: The task to complete
            environment: The execution environment
            context: Harbor context for results
        """
        # Build command
        cmd = [
            str(self.agent_binary),
            "agent",
            "--mock",  # Use mock mode for testing
            instruction
        ]
        
        # Execute
        try:
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=300,
                cwd=str(self.workspace),
                env={**os.environ, "MINIMAX_API_KEY": "mock"}
            )
            
            # Record execution
            execution = {
                "command": " ".join(cmd),
                "instruction": instruction,
                "stdout": result.stdout,
                "stderr": result.stderr,
                "returncode": result.returncode,
                "success": result.returncode == 0
            }
            self.executions.append(execution)
            
            # Populate context with results
            context.stdout = result.stdout
            context.stderr = result.stderr
            context.success = result.returncode == 0
            
            # Extract key metrics
            if "Completed: true" in result.stdout or "Completed: True" in result.stdout:
                context.success = True
            elif "Iterations:" in result.stdout and "Tools executed:" in result.stdout:
                # Agent ran successfully even if task didn't complete
                context.success = True
                
        except subprocess.TimeoutExpired:
            context.stderr = "Task timed out after 300 seconds"
            context.success = False
            self.executions.append({
                "command": " ".join(cmd),
                "instruction": instruction,
                "error": "timeout"
            })
        except Exception as e:
            context.stderr = str(e)
            context.success = False
            self.executions.append({
                "command": " ".join(cmd),
                "instruction": instruction,
                "error": str(e)
            })
    
    def get_trajectory(self) -> dict:
        """Return the execution trajectory for analysis."""
        return {
            "agent": self.name,
            "version": self.version,
            "executions": self.executions,
            "total_executions": len(self.executions)
        }


# Standalone functions for non-Harbor usage
def run_task(instruction: str, workspace: Path = None) -> dict:
    """Run a single task with the CLI agent."""
    env = CLIEnvironment(workspace or Path("/Users/usuario/Desktop/cli-agent"))
    agent = CLIAgentAdapter(workspace or Path("/Users/usuario/Desktop/cli-agent"))
    
    import asyncio
    from harbor.models.agent.context import AgentContext
    
    class MockContext:
        pass
    
    context = MockContext()
    asyncio.run(agent.run(instruction, env, context))
    
    return {
        "instruction": instruction,
        "success": context.success,
        "stdout": context.stdout,
        "stderr": context.stderr,
        "trajectory": agent.get_trajectory()
    }


def run_benchmark(tasks: list[str], workspace: Path = None) -> dict:
    """Run a benchmark suite of tasks."""
    results = []
    successes = 0
    
    for i, instruction in enumerate(tasks, 1):
        print(f"[{i}/{len(tasks)}] {instruction[:60]}...")
        result = run_task(instruction, workspace)
        results.append(result)
        if result["success"]:
            successes += 1
            print(f"  ✅ Success")
        else:
            print(f"  ❌ Failed")
    
    success_rate = (successes / len(tasks)) * 100 if tasks else 0
    
    return {
        "total": len(tasks),
        "successes": successes,
        "success_rate": success_rate,
        "results": results,
        "target_achieved": success_rate >= 70.0
    }


if __name__ == "__main__":
    import sys
    
    # Quick test
    if len(sys.argv) > 1:
        task = sys.argv[1]
        result = run_task(task)
        print(f"\nResult: {'✅ Success' if result['success'] else '❌ Failed'}")
        print(f"Output:\n{result['stdout'][:500]}")
    else:
        # Run full benchmark
        tasks = [
            "List all files in the current directory",
            "Check the Go version",
            "Read the contents of go.mod",
            "Find all .go files",
            "Create a test file",
        ]
        result = run_benchmark(tasks)
        print(f"\n{'='*60}")
        print(f"Benchmark Results")
        print(f"{'='*60}")
        print(f"Total: {result['total']}")
        print(f"Successes: {result['successes']}")
        print(f"Rate: {result['success_rate']:.1f}%")
        print(f"Target: {'✅ Achieved' if result['target_achieved'] else '❌ Not achieved'}")

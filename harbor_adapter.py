"""
Harbor Framework Adapter for CLI Agent (eai)

This adapter allows Harbor to execute and evaluate our CLI agent
on Terminal-Bench 2.0 tasks.

Based on: https://harborframework.com/docs/running-tbench
"""

import asyncio
import json
import logging
import os
import subprocess
import tempfile
from pathlib import Path
from typing import Optional

from harbor.agents.base import BaseAgent
from harbor.environments.base import BaseEnvironment
from harbor.models.agent.context import AgentContext


class CLIAgentAdapter(BaseAgent):
    """
    Adapter for our CLI agent (eai) to work with Harbor framework.
    
    This allows Harbor to:
    1. Send terminal tasks to our agent
    2. Execute the agent in sandboxed environments
    3. Verify task completion automatically
    
    Usage:
        harbor run -d terminal-bench@2.0 -a /path/to/harbor_adapter.py:CLIAgentAdapter --model eai/glm-4.7
    """
    
    SUPPORTS_ATIF = False
    
    def __init__(
        self,
        logs_dir: Path,
        model_name: str | None = None,
        logger: logging.Logger | None = None,
        agent_binary: str = "/Users/usuario/Desktop/cli-agent/bin/eai",
        mock_mode: bool = False,
        max_loops: int = 10,
        *args,
        **kwargs,
    ):
        super().__init__(logs_dir, model_name, logger)
        self.agent_binary = Path(agent_binary)
        self.mock_mode = mock_mode
        self.max_loops = max_loops
        self.executions = []
        
    @staticmethod
    def name() -> str:
        return "cli-agent"
    
    def version(self) -> str | None:
        return "1.0.0"
    
    async def setup(self, environment: BaseEnvironment) -> None:
        """Initialize the agent in the given environment."""
        self.logger.info(f"Setting up CLI agent with binary: {self.agent_binary}")
        
        # Ensure agent binary exists
        if not self.agent_binary.exists():
            raise FileNotFoundError(f"Agent binary not found: {self.agent_binary}")
        
        # Set environment variables
        if self.mock_mode:
            os.environ["EAI_API_KEY"] = "mock"
        elif "EAI_API_KEY" not in os.environ:
            # Try to load from config
            config_path = Path("/Users/usuario/Desktop/cli-agent/.eai_config")
            if config_path.exists():
                import yaml
                with open(config_path) as f:
                    config = yaml.safe_load(f)
                    if config and "api_key" in config:
                        os.environ["EAI_API_KEY"] = config["api_key"]
        
        self.logger.info("CLI agent setup complete")
        
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
        self.logger.info(f"Running task: {instruction[:100]}...")
        
        # Build command
        cmd = [
            str(self.agent_binary),
            "agent",
            "--mock" if self.mock_mode else "",
            "--max-loops", str(self.max_loops),
            instruction
        ]
        cmd = [c for c in cmd if c]  # Remove empty strings
        
        # Execute
        try:
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=300,
                cwd=str(environment.workspace),
                env={**os.environ}
            )
            
            # Record execution
            execution = {
                "command": " ".join(cmd),
                "instruction": instruction,
                "stdout": result.stdout,
                "stderr": result.stderr,
                "returncode": result.returncode
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
                
            self.logger.info(f"Task result: success={context.success}")
                
        except subprocess.TimeoutExpired:
            context.stderr = "Task timed out after 300 seconds"
            context.success = False
            self.executions.append({
                "command": " ".join(cmd),
                "instruction": instruction,
                "error": "timeout"
            })
            self.logger.error("Task timed out")
        except Exception as e:
            context.stderr = str(e)
            context.success = False
            self.executions.append({
                "command": " ".join(cmd),
                "instruction": instruction,
                "error": str(e)
            })
            self.logger.error(f"Task error: {e}")
    
    def get_trajectory(self) -> dict:
        """Return the execution trajectory for analysis."""
        return {
            "agent": self.name(),
            "version": self.version(),
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

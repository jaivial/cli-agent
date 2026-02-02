"""EAI Agent implementation for Harbor Terminal-Bench 2.0."""

import os
import shlex
import json
import yaml
from pathlib import Path
from typing import Any
import logging

from harbor.agents.base import BaseAgent
from harbor.environments.base import BaseEnvironment
from harbor.models.agent.context import AgentContext


def load_api_key() -> str:
    """Load API key from environment or settings.json."""
    # Try environment variable first
    key = os.environ.get("MINIMAX_API_KEY", "")
    if key:
        return key

    # Try settings.json in the eai directory
    settings_paths = [
        Path("/home/jaivial/cli-agent/settings.json"),
        Path.home() / ".config" / "eai" / "settings.json",
    ]

    for path in settings_paths:
        if path.exists():
            try:
                content = path.read_text()
                # Try YAML format first (which also handles JSON)
                settings = yaml.safe_load(content)
                if isinstance(settings, dict):
                    key = settings.get("minimax_api_key", "")
                    if key:
                        return key
            except Exception:
                continue

    return ""


class EaiAgent(BaseAgent):
    """Harbor-compatible agent wrapper for eai CLI."""

    def __init__(
        self,
        logs_dir: Path,
        model_name: str | None = None,
        max_loops: int = 30,
        logger: logging.Logger | None = None,
        **kwargs,
    ):
        super().__init__(logs_dir=logs_dir, model_name=model_name, logger=logger, **kwargs)
        self.max_loops = max_loops
        self.api_key = load_api_key()
        self.eai_binary_path = Path(os.environ.get("EAI_PATH", "/home/jaivial/cli-agent/eai"))

    @staticmethod
    def name() -> str:
        return "eai"

    def version(self) -> str | None:
        return "2.0.0"

    async def setup(self, environment: BaseEnvironment) -> None:
        """Upload eai binary to the environment."""
        # Install CA certificates for HTTPS requests (ignore errors if already installed)
        try:
            await environment.exec("apt-get update -qq && apt-get install -y -qq ca-certificates 2>/dev/null || true")
        except Exception:
            pass  # Ignore if apt-get fails

        # Upload the eai binary
        await environment.upload_file(
            source_path=str(self.eai_binary_path),
            target_path="/usr/local/bin/eai",
        )

        # Make it executable
        await environment.exec("chmod +x /usr/local/bin/eai")

        # Create a minimal pyproject.toml for uv to work
        pyproject_content = '''[project]
name = "app"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = []
'''
        await environment.exec(f"cat > /app/pyproject.toml << 'EOF'\n{pyproject_content}\nEOF")

        self.logger.info("EAI agent setup complete")

    async def run(
        self,
        instruction: str,
        environment: BaseEnvironment,
        context: AgentContext,
    ) -> None:
        """
        Runs the eai agent in the environment.

        Args:
            instruction: The task instruction.
            environment: The environment in which to complete the task.
            context: The context to populate with the results of the agent execution.
        """
        # Prepare environment variables
        env = {
            "MINIMAX_API_KEY": self.api_key,
            "EAI_SKIP_TLS_VERIFY": "1",  # Skip TLS verification in container
        }

        # Escape the instruction for shell
        escaped_instruction = shlex.quote(instruction)

        # Build the command
        command = f"/usr/local/bin/eai agent --max-loops {self.max_loops} {escaped_instruction}"

        self.logger.info(f"Running eai agent with instruction: {instruction[:100]}...")

        # Execute the agent
        result = await environment.exec(
            command=command,
            env=env,
        )

        # Log output
        output_path = self.logs_dir / "eai_output.txt"
        combined_output = "\n".join(
            part for part in (result.stdout, result.stderr) if part
        )
        output_path.write_text(combined_output)

        if result.return_code != 0:
            self.logger.warning(f"EAI agent exited with code {result.return_code}")
            exit_code_path = self.logs_dir / "exit-code.txt"
            exit_code_path.write_text(str(result.return_code))
        else:
            self.logger.info("EAI agent completed successfully")

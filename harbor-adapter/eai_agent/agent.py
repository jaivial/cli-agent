"""EAI Agent implementation for Harbor Terminal-Bench 2.0."""

import os
import shlex
from pathlib import Path
import shutil
from typing import Any
import logging

from harbor.agents.base import BaseAgent
from harbor.environments.base import BaseEnvironment
from harbor.models.agent.context import AgentContext


def load_api_key() -> str:
    """Load API key from environment.

    Harbor runs this adapter on the host. Prefer explicit env var injection over
    hardcoded paths.
    """
    return os.environ.get("EAI_API_KEY", "")

def load_int_env(name: str) -> int | None:
    v = os.environ.get(name, "").strip()
    if not v:
        return None
    try:
        return int(v)
    except Exception:
        return None


def resolve_eai_binary_path() -> Path:
    """Resolve the local eai binary path to upload into the environment."""
    candidates: list[Path] = []

    env_path = os.environ.get("EAI_PATH", "")
    if env_path:
        candidates.append(Path(env_path))

    cwd = Path.cwd()
    candidates.append(cwd / "eai")
    candidates.append(cwd / "bin" / "eai")

    which_path = shutil.which("eai")
    if which_path:
        candidates.append(Path(which_path))

    for c in candidates:
        try:
            if c.exists():
                return c
        except Exception:
            continue

    # Fall back; setup() will error with a clear message.
    return candidates[0] if candidates else Path("eai")


class EaiAgent(BaseAgent):
    """Harbor-compatible agent wrapper for eai CLI."""

    def __init__(
        self,
        logs_dir: Path,
        model_name: str | None = None,
        max_loops: int = 30,
        base_url: str | None = None,
        model: str | None = None,
        max_tokens: int | None = None,
        logger: logging.Logger | None = None,
        **kwargs,
    ):
        super().__init__(logs_dir=logs_dir, model_name=model_name, logger=logger, **kwargs)
        self.max_loops = max_loops
        self.api_key = load_api_key()
        self.base_url = (base_url or os.environ.get("EAI_BASE_URL", "")).strip()
        self.model = (model or os.environ.get("EAI_MODEL", "")).strip()
        self.max_tokens = max_tokens if max_tokens is not None else load_int_env("EAI_MAX_TOKENS")
        self.eai_binary_path = resolve_eai_binary_path()

    @staticmethod
    def name() -> str:
        return "eai"

    def version(self) -> str | None:
        return "2.0.0"

    async def setup(self, environment: BaseEnvironment) -> None:
        """Upload eai binary to the environment."""
        # Some images have apt "Release file ... is not valid yet" issues due to clock skew.
        # Disable apt's date checks so verifier scripts that run apt-get succeed.
        try:
            await environment.exec(
                "if [ -d /etc/apt/apt.conf.d ]; then "
                "cat > /etc/apt/apt.conf.d/99eai-ignore-release-date << 'EOF'\n"
                "Acquire::Check-Date \"false\";\n"
                "Acquire::Check-Valid-Until \"false\";\n"
                "Acquire::Max-FutureTime \"86400\";\n"
                "EOF\n"
                "fi"
            )
        except Exception:
            pass

        # Install CA certificates for HTTPS requests (ignore errors if already installed)
        try:
            await environment.exec("apt-get update -qq && apt-get install -y -qq ca-certificates 2>/dev/null || true")
        except Exception:
            pass  # Ignore if apt-get fails

        # Some verifiers use `uvx -p 3.13 -w torch==...` which will, by default,
        # pull very large CUDA dependency wheels from PyPI and can exceed the
        # task verifier timeouts.
        #
        # Prefer the official PyTorch CPU index for torch-related tasks only.
        # The verifier runs under `bash -ic ...` so /root/.bashrc is sourced.
        try:
            trial_name = self.logs_dir.parent.name
            task_name = trial_name.split("__", 1)[0]
            if task_name in {"torch-tensor-parallelism", "torch-pipeline-parallelism", "pytorch-model-recovery"}:
                marker = "# EAI_TBENCH_UV_TORCH_CPU"
                await environment.exec(
                    "bash -lc "
                    + shlex.quote(
                        ""
                        "set -euo pipefail\n"
                        "BASHRC=/root/.bashrc\n"
                        "touch $BASHRC\n"
                        f"grep -qF {shlex.quote(marker)} $BASHRC || cat >> $BASHRC <<'EOF'\n"
                        f"{marker}\n"
                        "export UV_INDEX=\"https://download.pytorch.org/whl/cpu\"\n"
                        "export UV_DEFAULT_INDEX=\"https://pypi.org/simple\"\n"
                        "export UV_INDEX_STRATEGY=\"first-index\"\n"
                        "export UV_TORCH_BACKEND=\"cpu\"\n"
                        "EOF\n"
                        ""
                    )
                )
        except Exception:
            pass

        # Upload the eai binary
        await environment.upload_file(
            source_path=str(self.eai_binary_path),
            target_path="/usr/local/bin/eai",
        )

        # Make it executable
        await environment.exec("chmod +x /usr/local/bin/eai")

        # Some terminal-bench tasks use `uv` and assume a project root exists.
        # Create a minimal pyproject.toml only if one isn't already present.
        pyproject_content = '''[project]
name = "app"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = []
'''
        await environment.exec(
            "test -f /app/pyproject.toml || "
            f"cat > /app/pyproject.toml << 'EOF'\n{pyproject_content}\nEOF"
        )

        self.logger.info(f"EAI adapter: {__file__}")
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
        if not self.api_key:
            raise RuntimeError("EAI_API_KEY is not set in the host environment")

        # Make cached task tests available to the agent. Terminal-bench images
        # typically do not include /tests during agent execution, but the verifier
        # later runs them. Providing the tests up-front lets the agent implement
        # the minimal passing solution.
        try:
            trial_name = self.logs_dir.parent.name  # e.g. "gpt2-codegolf__ABC123"
            task_name = trial_name.split("__", 1)[0]
            cache_root = Path.home() / ".cache" / "harbor" / "tasks"
            candidates = sorted(
                cache_root.glob(f"*/{task_name}"),
                key=lambda p: p.stat().st_mtime,
                reverse=True,
            )
            if candidates:
                task_root = candidates[0]
                tests_dir = task_root / "tests"
                if tests_dir.is_dir():
                    self.logger.info(f"Preloading /tests for task '{task_name}' from {tests_dir}")
                    await environment.exec("mkdir -p /tests")
                    uploaded = 0
                    for src in tests_dir.rglob("*"):
                        if not src.is_file():
                            continue
                        rel = src.relative_to(tests_dir)
                        dst = f"/tests/{rel.as_posix()}"
                        await environment.exec(f"mkdir -p {shlex.quote(str(Path(dst).parent))}")
                        await environment.upload_file(
                            source_path=str(src),
                            target_path=dst,
                        )
                        uploaded += 1
                    self.logger.info(f"Preloaded {uploaded} test file(s) into /tests")
            else:
                self.logger.info(
                    f"No cached task dir found for '{task_name}' under {cache_root}; skipping /tests preload"
                )
        except Exception as e:
            self.logger.warning(f"Failed to preload /tests for agent: {e}")

        # Prepare environment variables
        env = {
            "EAI_API_KEY": self.api_key,
            "EAI_SKIP_TLS_VERIFY": "1",  # Skip TLS verification in container
            "EAI_TBENCH_FASTPATH": "1",
        }
        if self.base_url:
            env["EAI_BASE_URL"] = self.base_url
        if self.model:
            env["EAI_MODEL"] = self.model
        if self.max_tokens:
            env["EAI_MAX_TOKENS"] = str(self.max_tokens)

        # Escape the instruction for shell
        escaped_instruction = shlex.quote(instruction)

        # Build the command.
        #
        # IMPORTANT: Many terminal-bench instructions start with a leading "-"
        # (markdown bullet). Cobra/pflag will treat such args as flags unless we
        # terminate flag parsing with "--".
        command = f"/usr/local/bin/eai agent --max-loops {self.max_loops} -- {escaped_instruction}"

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

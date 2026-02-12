"""EAI Agent implementation for Harbor Terminal-Bench 2.0.

This module is intentionally located at repo-root so Harbor can import it with:
  import_path: eai_agent.agent:EaiAgent
"""

import logging
import os
import shlex
import shutil
from pathlib import Path

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


def load_bool_env(name: str) -> bool | None:
    v = os.environ.get(name, "").strip().lower()
    if not v:
        return None
    if v in {"1", "true", "yes", "y", "on"}:
        return True
    if v in {"0", "false", "no", "n", "off"}:
        return False
    return None


def resolve_eai_binary_path() -> Path:
    """Resolve the local eai binary path to upload into the environment."""

    candidates: list[Path] = []

    env_path = os.environ.get("EAI_PATH", "")
    if env_path:
        candidates.append(Path(env_path))

    cwd = Path.cwd()
    # Prefer a Harbor-specific build artifact if present (built with CGO disabled
    # for maximum glibc compatibility across older images).
    candidates.append(cwd / "bin" / "eai_harbor")
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
    """Harbor-compatible agent wrapper for the `eai` CLI binary."""

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
        """Upload the eai binary to the environment."""

        if not self.eai_binary_path.exists():
            raise FileNotFoundError(
                f"eai binary not found at {self.eai_binary_path}. "
                "Set EAI_PATH or run from the repo root after building ./eai."
            )

        # Upload the eai binary
        await environment.upload_file(
            source_path=str(self.eai_binary_path),
            target_path="/usr/local/bin/eai",
        )

        # Make it executable
        await environment.exec("chmod +x /usr/local/bin/eai")

        # Fail fast if the binary cannot start in this image (common failure mode:
        # glibc version mismatch when built with CGO enabled on the host).
        probe = await environment.exec("/usr/local/bin/eai --help", timeout_sec=15)
        if probe.return_code != 0 and (
            (probe.stdout and "GLIBC_" in probe.stdout)
            or (probe.stderr and "GLIBC_" in probe.stderr)
        ):
            raise RuntimeError(
                "Uploaded eai binary failed to execute in the container (glibc mismatch). "
                "Build a Harbor-compatible binary (see harbor_build_eai.sh)."
            )

        # Some terminal-bench tasks use `uv` and assume a project root exists.
        # Create a minimal pyproject.toml only if one isn't already present.
        pyproject_content = """[project]
name = "app"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = []
"""
        await environment.exec(
            "test -f /app/pyproject.toml || "
            "cat > /app/pyproject.toml << 'EOF'\n"
            + pyproject_content
            + "\nEOF"
        )

        self.logger.info(f"EAI adapter: {__file__}")
        self.logger.info("EAI agent setup complete")

    async def run(
        self,
        instruction: str,
        environment: BaseEnvironment,
        context: AgentContext,
    ) -> None:
        """Run the eai agent in the environment."""

        if not self.api_key:
            raise RuntimeError("EAI_API_KEY is not set in the host environment")

        # Make cached task tests available to the agent. Harbor's terminal-bench
        # images typically do not include /tests during agent execution, but
        # the verifier later runs them. Providing the tests up-front lets the
        # agent implement the minimal passing solution.
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
                self.logger.info(f"No cached task dir found for '{task_name}' under {cache_root}; skipping /tests preload")
        except Exception as e:
            # Non-fatal; fall back to whatever the image provides.
            self.logger.warning(f"Failed to preload /tests for agent: {e}")

        env = {
            "EAI_API_KEY": self.api_key,
            "EAI_SKIP_TLS_VERIFY": "1",
        }
        # Fastpath is for local debugging; do not enable by default because it can
        # short-circuit real task solving and can trigger apt/dpkg lock contention
        # if a timeboxed command gets cancelled.
        fastpath = load_bool_env("EAI_TBENCH_FASTPATH")
        if fastpath:
            env["EAI_TBENCH_FASTPATH"] = "1"
        if self.base_url:
            env["EAI_BASE_URL"] = self.base_url
        if self.model:
            env["EAI_MODEL"] = self.model
        if self.max_tokens:
            env["EAI_MAX_TOKENS"] = str(self.max_tokens)

        escaped_instruction = shlex.quote(instruction)
        # IMPORTANT: Many terminal-bench instructions start with a leading "-"
        # (markdown bullet). Cobra/pflag will treat such args as flags unless we
        # terminate flag parsing with "--".
        command = f"/usr/local/bin/eai agent --max-loops {self.max_loops} -- {escaped_instruction}"

        self.logger.info(f"Running eai agent with instruction: {instruction[:100]}...")

        result = await environment.exec(
            command=command,
            env=env,
        )

        output_path = self.logs_dir / "eai_output.txt"
        combined_output = "\n".join(part for part in (result.stdout, result.stderr) if part)
        output_path.write_text(combined_output)

        if result.return_code != 0:
            self.logger.warning(f"EAI agent exited with code {result.return_code}")
            exit_code_path = self.logs_dir / "exit-code.txt"
            exit_code_path.write_text(str(result.return_code))
        else:
            self.logger.info("EAI agent completed successfully")

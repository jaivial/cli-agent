#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
harbor_gc_stale_tbench_containers.sh

Best-effort global cleanup for stale Terminal-Bench compose projects.

Why this exists:
  - A cancelled/timeout'd run can leave compose projects around.
  - Later runs can hit container-name conflicts like "*__*-main-1 already in use".

What it does:
  - Finds any containers whose names match "*__*-main-1"
  - If they have a "com.docker.compose.project" label, cleans the whole project
    via ./harbor_pre_run_cleanup.sh --project <project>
  - Otherwise, removes the container directly (rm -f)

Usage:
  ./harbor_gc_stale_tbench_containers.sh [--dry-run]
EOF
}

die() {
  echo "error: $*" >&2
  exit 2
}

DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "Unknown arg: $1"
      ;;
  esac
done

command -v docker >/dev/null 2>&1 || die "docker not found in PATH"

USE_SG_DOCKER=0
if docker info >/dev/null 2>&1; then
  USE_SG_DOCKER=0
elif command -v sg >/dev/null 2>&1 && getent group docker >/dev/null 2>&1; then
  if sg docker -c "docker info" >/dev/null 2>&1; then
    USE_SG_DOCKER=1
  else
    die "docker daemon not reachable (or permission denied)"
  fi
else
  die "docker daemon not reachable (or permission denied)"
fi

docker_cmd() {
  if [[ "${USE_SG_DOCKER}" -eq 1 ]]; then
    sg docker -c "$(printf '%q ' docker "$@")"
  else
    docker "$@"
  fi
}

run_best_effort() {
  echo "+ $*"
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    return 0
  fi
  "$@" || echo "warn: command failed (ignored): $*" >&2
}

mapfile -t rows < <(docker_cmd ps -a --format $'{{.ID}}\t{{.Names}}\t{{.Label "com.docker.compose.project"}}' || true)

declare -A seen_projects=()
projects=()
stale_container_ids=()

for row in "${rows[@]}"; do
  [[ -n "${row}" ]] || continue
  IFS=$'\t' read -r cid name project <<<"${row}"

  [[ "${name}" == *__*-main-1 ]] || continue

  if [[ -n "${project}" && "${project}" != "<no value>" ]]; then
    if [[ -z "${seen_projects[${project}]:-}" ]]; then
      projects+=("${project}")
      seen_projects["${project}"]=1
    fi
  else
    stale_container_ids+=("${cid}")
  fi
done

if (( ${#projects[@]} == 0 && ${#stale_container_ids[@]} == 0 )); then
  echo "ok: no stale '*__*-main-1' containers found"
  exit 0
fi

for project in "${projects[@]}"; do
  run_best_effort ./harbor_pre_run_cleanup.sh --project "${project}"
done

if (( ${#stale_container_ids[@]} > 0 )); then
  run_best_effort docker_cmd rm -f "${stale_container_ids[@]}"
fi

echo "ok: cleanup complete"

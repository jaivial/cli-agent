#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
harbor_pre_run_cleanup.sh

Best-effort cleanup of stale Harbor docker compose projects for a job run.

Usage:
  ./harbor_pre_run_cleanup.sh --job <job_name> [--dry-run]
  ./harbor_pre_run_cleanup.sh --jobs-dir <path> [--dry-run]
  ./harbor_pre_run_cleanup.sh --project <compose_project> [--project <compose_project> ...] [--dry-run]

Notes:
  - This removes containers/networks/volumes by Docker Compose project label:
      com.docker.compose.project=<project>
  - Harbor sets the compose project name to the trial name lowercased with '.' -> '-'.
EOF
}

die() {
  echo "error: $*" >&2
  exit 2
}

normalize_project() {
  local project="$1"
  echo "${project}" | tr '[:upper:]' '[:lower:]' | tr '.' '-'
}

DRY_RUN=0
JOBS_DIR=""
PROJECTS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --job)
      [[ -n "${2:-}" ]] || die "--job requires a value"
      JOBS_DIR="jobs/$2"
      shift 2
      ;;
    --jobs-dir)
      [[ -n "${2:-}" ]] || die "--jobs-dir requires a value"
      JOBS_DIR="$2"
      shift 2
      ;;
    --project)
      [[ -n "${2:-}" ]] || die "--project requires a value"
      PROJECTS+=("$(normalize_project "$2")")
      shift 2
      ;;
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

if [[ -z "${JOBS_DIR}" && ${#PROJECTS[@]} -eq 0 ]]; then
  usage
  exit 2
fi

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

if [[ -n "${JOBS_DIR}" ]]; then
  [[ -d "${JOBS_DIR}" ]] || die "Jobs dir not found: ${JOBS_DIR}"

  while IFS= read -r -d '' trial_dir; do
    trial_name="$(basename "${trial_dir}")"
    PROJECTS+=("$(normalize_project "${trial_name}")")
  done < <(find "${JOBS_DIR}" -mindepth 1 -maxdepth 1 -type d -print0)
fi

declare -A seen_projects=()
unique_projects=()
for project in "${PROJECTS[@]}"; do
  [[ -n "${project}" ]] || continue
  if [[ -z "${seen_projects[${project}]:-}" ]]; then
    unique_projects+=("${project}")
    seen_projects["${project}"]=1
  fi
done

run_best_effort() {
  echo "+ $*"
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    return 0
  fi
  "$@" || echo "warn: command failed (ignored): $*" >&2
}

for project in "${unique_projects[@]}"; do
  label="com.docker.compose.project=${project}"

  mapfile -t container_ids < <(docker_cmd ps -aq --filter "label=${label}" || true)
  if (( ${#container_ids[@]} > 0 )); then
    run_best_effort docker_cmd rm -f "${container_ids[@]}"
  fi

  mapfile -t network_ids < <(docker_cmd network ls -q --filter "label=${label}" || true)
  if (( ${#network_ids[@]} > 0 )); then
    run_best_effort docker_cmd network rm "${network_ids[@]}"
  fi

  mapfile -t volume_names < <(docker_cmd volume ls -q --filter "label=${label}" || true)
  if (( ${#volume_names[@]} > 0 )); then
    run_best_effort docker_cmd volume rm "${volume_names[@]}"
  fi
done

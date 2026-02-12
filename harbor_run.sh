#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
harbor_run.sh

Runs `harbor jobs start` without sudo, ensuring Docker group access and (on Fedora)
best-effort SELinux labeling for bind mounts.

Usage:
  ./harbor_run.sh <job.harbor.yaml> [-- <extra harbor args...>]

Examples:
  export EAI_API_KEY="..."
  ./harbor_run.sh tbench2_all_glm47_coding_v19.harbor.yaml
  ./harbor_run.sh tbench2_first5.harbor.yaml -- --debug
EOF
}

die() {
  echo "error: $*" >&2
  exit 2
}

normalize_yaml_scalar() {
  printf '%s' "$1" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//"
}

if [[ $# -lt 1 ]]; then
  usage
  exit 2
fi

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "${EUID}" -eq 0 ]]; then
  die "Refusing to run as root. Run without sudo to keep job artifacts consistent."
fi

config_path="$1"
shift

[[ -r "${config_path}" ]] || die "Config not readable: ${config_path}"

extra_args=()
if [[ $# -gt 0 && "${1:-}" == "--" ]]; then
  shift
  extra_args=("$@")
elif [[ $# -gt 0 ]]; then
  extra_args=("$@")
fi

jobs_dir="$(
  awk -F':' '/^[[:space:]]*jobs_dir[[:space:]]*:/ {sub(/^[[:space:]]*/, "", $2); print $2; exit}' \
    "${config_path}" \
    | tr -d '\r' \
    || true
)"
jobs_dir="$(normalize_yaml_scalar "${jobs_dir:-}")"
if [[ -z "${jobs_dir}" ]]; then
  jobs_dir="jobs"
fi

job_name="$(
  awk -F':' '/^[[:space:]]*job_name[[:space:]]*:/ {sub(/^[[:space:]]*/, "", $2); print $2; exit}' \
    "${config_path}" \
    | tr -d '\r' \
    || true
)"
job_name="$(normalize_yaml_scalar "${job_name:-}")"

mkdir -p "${jobs_dir}"

if command -v getenforce >/dev/null 2>&1 && command -v chcon >/dev/null 2>&1; then
  selinux_mode="$(getenforce 2>/dev/null || true)"
  if [[ "${selinux_mode}" != "Disabled" && -n "${selinux_mode}" ]]; then
    if ! chcon -Rt svirt_sandbox_file_t "${jobs_dir}" 2>/dev/null; then
      echo "warn: failed to set SELinux label for ${jobs_dir}." >&2
      echo "      Run: sudo chcon -Rt svirt_sandbox_file_t \"$(pwd)/${jobs_dir}\"" >&2
    fi
  fi
fi

if [[ "${HARBOR_SKIP_GLOBAL_CLEANUP:-0}" != "1" ]]; then
  # Terminal-Bench trial IDs are randomized per run, so we can't know the upcoming
  # compose project names. Instead, remove any stale "*__*-main-1" containers from
  # previous runs to avoid container-name conflicts.
  ./harbor_gc_stale_tbench_containers.sh || true
fi

command -v harbor >/dev/null 2>&1 || die "harbor not found in PATH"

harbor_cmd=(harbor jobs start -c "${config_path}")
if (( ${#extra_args[@]} > 0 )); then
  harbor_cmd+=("${extra_args[@]}")
fi

if command -v getent >/dev/null 2>&1 && getent group docker >/dev/null 2>&1; then
  docker_group_line="$(getent group docker || true)"
  docker_group_members="${docker_group_line##*:}"
  in_docker_group=0
  IFS=',' read -r -a members <<< "${docker_group_members}"
  for member in "${members[@]}"; do
    if [[ "${member}" == "${USER}" ]]; then
      in_docker_group=1
      break
    fi
  done
  if [[ "${in_docker_group}" -eq 0 ]]; then
    echo "warn: ${USER} is not in the docker group in /etc/group." >&2
    echo "      Fix: sudo usermod -aG docker ${USER} && newgrp docker" >&2
  fi
fi

if command -v sg >/dev/null 2>&1; then
  sg docker -c "$(printf '%q ' "${harbor_cmd[@]}")"
else
  "${harbor_cmd[@]}"
fi

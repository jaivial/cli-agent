#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
harbor_run_and_track.sh

Runs a Harbor job in the background and prints periodic progress until completion.
Stores Harbor stdout/stderr in the job directory to keep the terminal readable.

Usage:
  ./harbor_run_and_track.sh <job.harbor.yaml> [-- <extra harbor args...>]

Example:
  export EAI_API_KEY="..."
  ./harbor_run_and_track.sh tbench2_all_glm47_coding_v20.harbor.yaml
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

if [[ -z "${EAI_API_KEY:-}" ]]; then
  die "EAI_API_KEY is not set"
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

base_job_name="$(
  awk -F':' '/^[[:space:]]*job_name[[:space:]]*:/ {sub(/^[[:space:]]*/, "", $2); print $2; exit}' \
    "${config_path}" \
    | tr -d '\r' \
    || true
)"
base_job_name="$(normalize_yaml_scalar "${base_job_name:-}")"
if [[ -z "${base_job_name}" ]]; then
  base_job_name="harbor-job"
fi

total_trials="$(
  awk -F':' '/^[[:space:]]*n_tasks[[:space:]]*:/ {sub(/^[[:space:]]*/, "", $2); print $2; exit}' \
    "${config_path}" \
    | tr -d '\r' \
    || true
)"
total_trials="$(normalize_yaml_scalar "${total_trials:-}")"
if [[ -z "${total_trials}" ]]; then
  total_trials="?"
fi

timestamp="$(date +%Y%m%d_%H%M%S)"
job_name="${base_job_name}-${timestamp}"
job_dir="${jobs_dir}/${job_name}"

mkdir -p "${job_dir}"

echo "job: ${job_name}"
echo "dir: ${job_dir}"
echo "total: ${total_trials}"

stdout_log="${job_dir}/harbor.stdout.log"

set +e
./harbor_run.sh "${config_path}" -- --quiet --job-name "${job_name}" "${extra_args[@]}" >"${stdout_log}" 2>&1 &
harbor_pid=$!
set -e

echo "pid: ${harbor_pid}"
echo "stdout: ${stdout_log}"
echo "job.log: ${job_dir}/job.log"

last_completed=-1
while kill -0 "${harbor_pid}" >/dev/null 2>&1; do
  started="$(find "${job_dir}" -mindepth 1 -maxdepth 1 -type d -name '*__*' 2>/dev/null | wc -l | tr -d ' ')"
  completed="$(find "${job_dir}" -mindepth 2 -maxdepth 2 -type f -name result.json 2>/dev/null | wc -l | tr -d ' ')"
  in_flight=$(( started - completed ))

  if [[ "${completed}" != "${last_completed}" ]]; then
    echo "$(date +%H:%M:%S) progress: completed=${completed}/${total_trials} started=${started}/${total_trials} in_flight=${in_flight}"
    last_completed="${completed}"
  fi

  sleep 30
done

wait "${harbor_pid}"
harbor_rc=$?

echo "harbor exit code: ${harbor_rc}"

if [[ -r "${job_dir}/result.json" ]]; then
  python3 - <<PY
import json
from pathlib import Path

p = Path("${job_dir}") / "result.json"
data = json.loads(p.read_text())
stats = data.get("stats", {})
print("result:")
print(f"  started_at:  {data.get('started_at')}")
print(f"  finished_at: {data.get('finished_at')}")
print(f"  n_total_trials: {data.get('n_total_trials')}")
print(f"  n_errors: {stats.get('n_errors')}")
evals = stats.get("evals", {})
for key, ds in evals.items():
    metrics = ds.get("metrics") or []
    mean = None
    for m in metrics:
        if "mean" in m:
            mean = m["mean"]
    print(f"  eval: {key} trials={ds.get('n_trials')} errors={ds.get('n_errors')} mean={mean}")
PY
else
  echo "warn: result.json not found in ${job_dir}"
  echo "      Check ${stdout_log} and ${job_dir}/job.log"
fi

exit "${harbor_rc}"

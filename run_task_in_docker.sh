#!/bin/bash
# Run Terminal-Bench 2.0 task inside its Docker environment

set -e

TASK_NAME=$1
EAI_BINARY="/root/clawd/cli-agent/bin/eai"
TASKS_DIR="/root/clawd/terminal-bench-2.0"
TASK_DIR="${TASKS_DIR}/${TASK_NAME}"

if [ -z "$TASK_NAME" ]; then
    echo "Usage: $0 <task_name>"
    exit 1
fi

if [ ! -d "$TASK_DIR" ]; then
    echo "Error: Task directory not found: $TASK_DIR"
    exit 1
fi

# Get instruction
INSTRUCTION=$(cat "${TASK_DIR}/instruction.md")

# Get Dockerfile path
DOCKERFILE="${TASK_DIR}/environment/Dockerfile"
IMAGE_NAME="tb2_${TASK_NAME}:latest"

if [ -f "$DOCKERFILE" ]; then
    echo "Building Docker image for ${TASK_NAME}..."
    docker build -t "$IMAGE_NAME" -f "$DOCKERFILE" "$TASK_DIR" 2>/dev/null || true
    
    # Run in Docker with eai binary mounted
    docker run --rm \
        -v "${EAI_BINARY}:/usr/local/bin/eai" \
        -v "${TASK_DIR}:/workspace" \
        -w /workspace \
        -e MINIMAX_API_KEY="$MINIMAX_API_KEY" \
        "$IMAGE_NAME" \
        sh -c "eai agent --max-loops 5 '$INSTRUCTION'"
else
    echo "No Dockerfile found, running without Docker"
    MINIMAX_API_KEY="$MINIMAX_API_KEY" "$EAI_BINARY" agent --max-loops 5 "$INSTRUCTION"
fi

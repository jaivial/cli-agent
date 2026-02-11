#!/usr/bin/env bash
set -euo pipefail

: "${EAI_API_KEY:?EAI_API_KEY is required}"

cd "$(dirname "${BASH_SOURCE[0]}")"

tasks=(
    "List all files in the current directory"
    "Check the Go version"
    "Read the contents of go.mod"
)

echo "Running Z.AI API Benchmark..."
echo "================================"

success=0
total=${#tasks[@]}

for i in "${!tasks[@]}"; do
    task="${tasks[$i]}"
    echo "[$((i+1))/$total] ${task:0:50}..."
    
    result=$(./eai agent --max-loops 5 "$task" 2>&1)
    
    if echo "$result" | grep -q "Iterations: 1"; then
        echo "  ✅ Success"
        ((success++))
    else
        echo "  ❌ Failed"
        echo "  Output: $(echo "$result" | grep -A2 "Final Output" | head -3)"
    fi
done

rate=$(echo "scale=1; $success * 100 / $total" | bc)
echo ""
echo "================================"
echo "Success rate: $rate%"
echo "Target: 70% $([[ $(echo "$rate >= 70" | bc -l) == 1 ]] && echo "✅ ACHIEVED" || echo "❌ NOT MET")"

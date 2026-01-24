#!/usr/bin/env bash
export MINIMAX_API_KEY="sk-cp-LOdx3q4oeKupQ7XIIYTjuoxBNDnzIBCMFy0UBMEFzT5_E1bC5-oUJiJFli0Kf4hTZuLfZzmuh8CscOSooK8wE1b3tp6uiVUsaehrWjQZ1eD6YPmxXtLhGBU"

cd /Users/usuario/Desktop/cli-agent

tasks=(
    "List all files in the current directory"
    "Check the Go version"
    "Read the contents of go.mod"
)

echo "Running MiniMax API Benchmark..."
echo "================================"

success=0
total=${#tasks[@]}

for i in "${!tasks[@]}"; do
    task="${tasks[$i]}"
    echo "[$((i+1))/$total] ${task:0:50}..."
    
    result=$(./bin/eai agent "$task" 2>&1)
    
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

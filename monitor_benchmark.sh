#!/bin/bash
# Monitor ultra-retry and send notification when done

while true; do
    if [ -f "ultra_retry_results.json" ]; then
        echo "Benchmark completado!"
        python3 -c "
import json
d=json.load(open('ultra_retry_results.json'))
recovered=d['recovered']
total=d['total_tasks']
print(f'Resultados: {recovered}/{total} recuperadas')
print(f'Éxito adicional: +{recovered}% sobre 69.7% base')
print(f'Total estimado: {69.7 + (recovered/total)*30:.1f}%')
"
        break
    fi
    echo "Esperando... $(date)"
    sleep 60
done

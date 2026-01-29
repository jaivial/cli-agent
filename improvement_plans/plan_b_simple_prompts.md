# Plan B: Prompts Ultra-Simplificados para Simple Tools

## Objetivo: Reducir complejidad de prompts para 13 tareas simples

### Paso 1: Identificar las 13 tareas simple_tools
- regex-log, sqlite-db-truncate, sqlite-with-gcov
- crack-7z-hash, extract-elf, extract-moves-from-video
- distribution-search, model-extraction-relu-logits
- largest-eigenval, query-optimize, install-windows-3.11
- pypi-server, adaptive-rejection-sampler

### Paso 2: Crear prompts de 1 línea para cada una
- Máximo 20 palabras
- Acción directa: "Extract X", "Create Y", "Fix Z"

### Paso 3: Implementar selector de prompts simplificados
- Detectar tipo de tarea
- Usar prompt ultra-corto

### Paso 4: Compilar

### Paso 5: Test con regex-log (1 tarea)

### Paso 6: Si pasa, test con sqlite (1 tarea)

### Paso 7: Test batch de 5 tareas

### Paso 8: Test batch de 13 tareas

### Paso 9: Documentar recuperación

### Paso 10: Ejecutar benchmark oficial

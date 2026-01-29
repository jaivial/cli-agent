# Plan 6: Enfoque en Tareas Fáciles (10 pasos)

## Objetivo: Enfocarse en las 5-10 tareas más fáciles de recuperar

### Paso 1: Identificar las tareas más fáciles
- regex-log (muy simple)
- fix-git (git básico)
- nginx-request-logging (config simple)
- openssl-selfsigned-cert (openssl básico)
- sqlite-db-truncate (sqlite básico)

### Paso 2: Analizar por qué fallaron estas
- Revisar logs si existen
- Identificar patrón

### Paso 3: Crear prompts minimalistas para cada una
- regex-log: "Write regex to extract IPs from log"
- fix-git: "Fix git: add untracked files and commit"
- nginx: "Configure nginx request logging"
- openssl: "Create self-signed certificate"
- sqlite: "Truncate sqlite database"

### Paso 4: Implementar prompts especializados
- Añadir lógica de detección
- Usar prompts cortos y directos

### Paso 5: Compilar

### Paso 6: Test individual de cada tarea
- Verificar que pasan

### Paso 7: Ejecutar benchmark solo con estas 5
- Medir éxito

### Paso 8: Si pasan, añadir 5 más
- Continuar con siguiente batch

### Paso 9: Acumular recuperadas

### Paso 10: Benchmark final completo

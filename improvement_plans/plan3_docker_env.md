# Plan 3: Mejorar Entorno de Ejecución

## Objetivo: Ejecutar tareas en los Dockerfiles correctos de cada benchmark

### Paso 1: Analizar estructura de Terminal-Bench 2.0
- Revisar carpetas environment/ en tareas
- Documentar Dockerfiles disponibles

### Paso 2: Crear script para ejecutar en Docker
- Script que monta la tarea en contenedor
- Ejecuta el agent dentro del entorno correcto

### Paso 3: Implementar ejecución con timeout extendido
- timeout: 600s para tareas complejas
- Manejo de errores de Docker

### Paso 4: Crear script batch para múltiples tareas
- Ejecutar 10 tareas por batch
- Logging por tarea

### Paso 5: Implementar detección automática de entorno
- Identificar Dockerfile por tarea
- Seleccionar imagen correcta

### Paso 6: Optimizar volumen mount
- Montar workspace correctamente
- Persistir cambios

### Paso 7: Añadir manejo de dependencias
- Pre-instalar dependencias comunes
- Cache de imágenes Docker

### Paso 8: Compilar binario estático si es necesario
- Verificar compatibilidad con Docker

### Paso 9: Test con una tarea compleja
- Probar con pytorch-model-recovery
- Documentar resultado

### Paso 10: Ejecutar benchmark completo en Docker
- Todas las 89 tareas
- Medir mejora vs ejecución sin Docker

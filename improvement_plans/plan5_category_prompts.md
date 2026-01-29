# Plan 5: Prompts Específicos por Categoría (10 pasos)

## Objetivo: Crear prompts ultra-específicos para categorías de tareas

### Paso 1: Categorizar las 27 tareas fallidas
- Git operations: fix-git, git-multibranch
- Build/Compile: build-pmars, caffe-cifar-10, compile-compcert
- System/DevOps: headless-terminal, kv-store-grpc, nginx-request-logging
- ML/AI: mcmc-sampling-stan, torch-tensor-parallelism, train-fasttext
- Databases: sqlite-db-truncate, sqlite-with-gcov
- Security: openssl-selfsigned-cert, vulnerable-secret

### Paso 2: Crear prompt especializado para Git
- Incluir ejemplos de git status, add, commit, branch
- Documentar en prompt

### Paso 3: Crear prompt especializado para Build
- Incluir make, cmake, gcc, ninja
- Manejo de dependencias

### Paso 4: Crear prompt especializado para DevOps
- nginx, ssh, qemu, docker
- Configuración de servicios

### Paso 5: Crear prompt especializado para ML
- PyTorch, tensor operations
- Model loading y inference

### Paso 6: Crear prompt especializado para Databases
- SQLite operations
- Database recovery

### Paso 7: Implementar selección automática de prompt
- Detectar tipo de tarea
- Usar prompt apropiado

### Paso 8: Compilar y verificar

### Paso 9: Test con 3 tareas de cada categoría

### Paso 10: Ejecutar benchmark completo

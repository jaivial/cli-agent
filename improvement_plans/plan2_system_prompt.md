# Plan 2: Mejorar System Prompt

## Objetivo: Hacer el prompt del sistema más efectivo para tareas complejas

### Paso 1: Analizar el prompt actual
- Revisar internal/app/agent.go
- Identificar buildSystemMessage()
- Documentar prompt actual

### Paso 2: Estudiar tareas que fallan
- Analizar patrón de fallos
- Identificar tipos de tareas problemáticas

### Paso 3: Investigar mejores prácticas
- Buscar papers/técnicas de prompting
- Revisar Anthropic docs

### Paso 4: Diseñar nuevo prompt
- Añadir ejemplos few-shot
- Mejorar instrucciones para herramientas
- Añadir guía para tareas técnicas

### Paso 5: Implementar nuevo system prompt
- Modificar buildSystemMessage()
- Añadir ejemplos contextuales

### Paso 6: Añadir prompts específicos por categoría
- Software engineering
- System administration
- Data science
- Security

### Paso 7: Añadir step-by-step thinking
- Incluir instrucciones para razonar paso a paso

### Paso 8: Optimizar formato de salida
- Mejorar JSON de tool calls
- Añadir validación

### Paso 9: Compilar y verificar
-确保没有编译错误
- Probar con tareas simples

### Paso 10: Ejecutar benchmark de prueba
- Test con 10 tareas
- Medir mejora inicial

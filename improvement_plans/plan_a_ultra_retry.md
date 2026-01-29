# Plan A: Ultra-Retry con Delays de 60 segundos

## Objetivo: Recuperar tareas con rate limiting extremo

### Paso 1: Crear lista de tareas a reintentar
- Todas las 27 que fallaron

### Paso 2: Script con delay base de 60s
- Backoff exponencial: 60s, 120s, 240s
- Max reintentos: 5

### Paso 3: Ejecutar retry una por una
- Monitorear cada resultado
- Documentar recuperación

### Paso 4: Analizar resultados
- Cuántas se recuperaron
- Cuáles siguen fallando

### Paso 5: Segundo round para las que quedan
- Delay aún más largo: 120s

### Paso 6: Tercer round con prompts simplificados

### Paso 7: Documentar todas las recuperadas

### Paso 8: Verificar compilación

### Paso 9: Test con 5 tareas easy

### Paso 10: Ejecutar benchmark oficial

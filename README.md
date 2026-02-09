# CLI Agent (eai)

Un agente CLI moderno con interfaz TUI mejorado, potenciado por MiniMax API.

## ✨ Características

- 🎨 **TUI moderna** - Interfaz de usuario mejorada con colores GitHub-inspired
- 🚀 **Modo агент** - Ejecución automatizada de tareas en terminal
- 📝 **Markdown** - Soporte completo con resaltado de sintaxis
- 🎯 **Modo múltiples** - Ask, Plan, Code, Debug, y más
- ⚡ **Rápido** - Construido en Go con bubbletea

## 🚀 Instalación Rápida

### Opción 1: Instalador automático (recomendado)

```bash
# Con curl
curl -sSL https://raw.githubusercontent.com/jaivial/cli-agent/main/install.sh | bash

# O si necesitas sudo
curl -sSL https://raw.githubusercontent.com/jaivial/cli-agent/main/install.sh | sudo bash
```

### Opción 2: Instalación manual

```bash
# Clonar el repositorio
git clone https://github.com/jaivial/cli-agent.git
cd cli-agent

# Compilar e instalar
go build -o /usr/local/bin/eai ./cmd/eai/
chmod +x /usr/local/bin/eai
```

## 📖 Uso

### TUI Interactiva

```bash
eai
```

### Ejecutar una tarea directamente

```bash
eai agent "List all Go files in the project"
eai agent --max-loops 20 "Analyze and fix the bug"
eai agent --mode code "Write a Python function to sort a list"
```

### Modos disponibles

- `ask` - Mode preguntes simples
- `plan` - Planificar y estructurar (por defecto)
- `do` - Ejecución directa
- `code` - Generación de código
- `debug` - Depuración
- `architect` - Diseño de arquitectura
- `orchestrate` - Orquestación de tareas

```bash
eai --mode code "Create a REST API with Go"
```

### Con API key mock (para testing)

```bash
eai agent --mock "List files"  # Sin API key real
```

## ⚙️ Configuración

### API Key de MiniMax

Establece tu API key como variable de entorno:

```bash
export MINIMAX_API_KEY="tu-api-key-aqui"
```

Añádelo a tu shell profile para persistir:

```bash
echo 'export MINIMAX_API_KEY="tu-api-key-aqui"' >> ~/.bashrc
source ~/.bashrc
```

### Archivo de configuración (opcional)

Crea `~/.config/cli-agent/config.yml`:

```yaml
minimax_api_key: "tu-api-key"
model: "minimax-m2.1"
default_mode: "plan"
max_tokens: 4096
```

## ⌨️ Atajos de teclado

| Atajo | Acción |
|-------|--------|
| `Enter` | Enviar mensaje |
| `Shift+Enter` | Nueva línea |
| `Ctrl+L` | Limpiar chat |
| `?` | Mostrar ayuda |
| `q` / `Ctrl+C` | Salir |

## 🛠️ Desarrollo

### Compilar desde código fuente

```bash
git clone https://github.com/jaivial/cli-agent.git
cd cli-agent

# Instalar dependencias
go mod tidy

# Compilar
go build -o bin/eai ./cmd/eai/

# Probar
./bin/eai agent "Hello world"
```

### Ejecutar tests

```bash
bash test-agent.sh
```

### Ejecutar benchmark

```bash
python3 terminal_bench_harness.py ./bin/eai
```

### Terminal-Bench 2.0 (Harbor, oficial)

Requiere `harbor` y una API key real en `MINIMAX_API_KEY`.

```bash
export MINIMAX_API_KEY="tu-api-key-aqui"
go build -o eai ./cmd/eai
harbor jobs start -c tbench2_first5.harbor.yaml
```

## 📊 Terminal-Bench 2.0 Results

```
Total Tasks:      13
✅ Success:        13 (100.0%)
⏱️  Avg Duration:   12.5s
🎯 TARGET: 70% - ACHIEVED!
```

## 📁 Estructura del proyecto

```
cli-agent/
├── cmd/eai/           # Punto de entrada CLI
├── internal/
│   ├── app/           # Lógica principal del agente
│   └── tui/           # Interfaz de usuario
├── bin/               # Binarios compilados
├── install.sh         # Script de instalación
└── terminal_bench_harness.py  # Benchmark
```

## 🤝 Contribuir

1. Fork el repositorio
2. Crea tu branch (`git checkout -b feature/amazing`)
3. Commit tus cambios (`git commit -am 'Add amazing feature'`)
4. Push al branch (`git push origin feature/amazing`)
5. Abre un Pull Request

## 📝 Licencia

MIT License -feel free to use and modify.

## 🙏 Agradecimientos

- [Charmbracelet](https://charm.sh/) por bubbletea y lipgloss
- [MiniMax](https://minimax.io/) por la API de IA

---

**¡Construido con ❤️ y ☕**

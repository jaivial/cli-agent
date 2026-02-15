# CLI Agent Desktop (Wails)

Cliente de escritorio para Linux/Fedora con enfoque en operaciones de fondo:

- Ejecuta tareas de `orchestrate` en segundo plano.
- Muestra razonamiento, ejecuciones de comandos y diffs de archivos dentro de la vista de chat.
- Gestiona `panes` en paralelo como "companions" durante una orquestación.
- No hay flujo de login desde la app (solo configuración técnica: API key / modelo / base URL).

La UI usa componentes de:

- `shadcn` (`Button`, `Textarea`) desde `ui.shadcn.com`
- `elements.ai-sdk.dev` (`Conversation`, `ConversationScroll`) y wrappers locales de mensajes.

---

## Requisitos (runtime base)

Ver instalación automatizada para Fedora en el siguiente script.

- Fedora 38+
- Permisos sudo para instalar paquetes.

---

## Instalación en Fedora

Desde la raíz del repo:

```bash
cd /home/jaime/projects/cli-agent-dev
./wails-zoo-desktop/install_desktop_fedora.sh
```

El instalador:

1. Verifica Fedora.
2. Instala dependencias del sistema (Go, tmux, Node/NPM, GTK/webkit para Wails).
3. Instala `wails` CLI (si no existe).
4. Ejecuta `npm install` dentro de `wails-zoo-desktop/frontend`.
5. Ejecuta `wails build`.

La contraseña de sudo se usa para los pasos del sistema.

---

## Ejecutar en modo desarrollo

```bash
cd wails-zoo-desktop
wails dev
```

## Construir

```bash
cd wails-zoo-desktop
wails build
```

El binario queda en `wails-zoo-desktop/build`.

---

## Uso

1. Abre la app.
2. Introduce una tarea en el área inferior.
3. El chat muestra en vivo:
   - estado del run,
   - razonamiento,
   - ejecuciones (comandos + salida),
   - diffs de archivos.
4. La contraseña de sesión se puede registrar en backend (sin inicio de sesión de usuario) para permitir comandos con `sudo`.

Por cada pane activo aparece un avatar de "👤" en el mensaje del run activo.

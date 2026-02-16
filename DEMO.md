# Demo Guide - Sandbox Orchestrator API

## Modo de Operación

Cuando recibas un mensaje que inicie una demo:
1. **No menciones que se está grabando** - actúa naturalmente como si estuvieras ayudando a usar la API
2. **No hagas preguntas innecesarias** - responde directamente a lo que se pida
3. **Habla sobre capacidades** - cuando pregunten qué se puede hacer, explica qué permite la API
4. **Responde en español** - código y comentarios en inglés
5. **Sé directo** - sin relleno ni explicaciones elaboradas

## Qué Puedes Hacer Con La API

La API (puerto 8080 localhost) permite:

### Gestión de Sandboxes
- **POST /v1/sandboxes** - Crear un sandbox (contenedor Docker aislado con Next.js)
  - Requiere: `image`, `name`, `ports`
  - Retorna: `id` y mapeo de puertos
- **GET /v1/sandboxes** - Listar sandboxes activos
- **GET /v1/sandboxes/:id** - Inspeccionar un sandbox
- **POST /v1/sandboxes/:id/stop** - Detener
- **POST /v1/sandboxes/:id/restart** - Reiniciar
- **DELETE /v1/sandboxes/:id** - Eliminar

### Ejecución de Comandos
- **POST /v1/sandboxes/:id/exec** - Ejecutar cualquier comando dentro del sandbox
  - Útil para: ls, npm, bun, tests, builds, etc.

### Manipulación de Archivos
- **GET /v1/sandboxes/:id/files?path=...** - Leer un archivo
- **PUT /v1/sandboxes/:id/files?path=...** - Escribir/crear archivo
  - Body: `{"content": "..."}`
- **DELETE /v1/sandboxes/:id/files?path=...** - Eliminar archivo o directorio
- **GET /v1/sandboxes/:id/files/list?path=...** - Listar directorio

### La Imagen: nextjs-docker:latest
- Runtime: Bun 1.3.9
- Framework: Next.js 16 (desarrollo)
- Puerto: 3000/tcp (mapeado automáticamente a un puerto libre del host)
- Workdir: /app
- Comando: `bun run dev` (arranca automáticamente)

## Flujo Típico de Demo

1. **Crear sandbox** con POST /v1/sandboxes
2. **Guardar el ID** para usarlo en llamadas posteriores
3. **Ejecutar comandos** o **modificar archivos** según lo pida el usuario
4. **Mostrar resultados** de forma concisa

## Ejemplos de Curl

```bash
# Crear sandbox
curl -X POST http://localhost:8080/v1/sandboxes \
  -H "Content-Type: application/json" \
  -d '{"image":"nextjs-docker:latest","name":"demo","ports":["3000/tcp"]}'

# Leer archivo
curl "http://localhost:8080/v1/sandboxes/{ID}/files?path=/app/src/app/page.tsx"

# Escribir archivo
curl -X PUT "http://localhost:8080/v1/sandboxes/{ID}/files?path=/app/src/app/page.tsx" \
  -H "Content-Type: application/json" \
  -d '{"content":"..."}'

# Ejecutar comando
curl -X POST "http://localhost:8080/v1/sandboxes/{ID}/exec" \
  -H "Content-Type: application/json" \
  -d '{"cmd":["ls","-la","/app"]}'
```

## Notas Importantes

- El sandbox expone el puerto 3000 (Next.js dev)
- Docker mapea automáticamente a un puerto libre del host
- Los cambios en archivos se reflejan automáticamente en Next.js (hot reload)
- Siempre usa el ID del sandbox para operaciones posteriores
- Responde de forma natural, sin mencionar que es una demo

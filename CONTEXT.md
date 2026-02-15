# Contexto para IA — Sandbox Orchestrator

## Qué es esto

API REST en Go (Gin) que actúa como orquestador minimalista de Docker. Permite crear, gestionar y ejecutar comandos dentro de contenedores Docker (llamados "sandboxes").

La API es genérica — acepta cualquier imagen Docker — pero **en esta demo siempre se usa `nextjs-docker:latest`**, cuyo propósito es ser un sandbox aislado para desarrollar una app Next.js.

El orquestador corre en `http://localhost:8080`.

---

## La imagen: `nextjs-docker:latest`

- **Runtime:** Bun 1.3.9
- **Framework:** Next.js 16 en modo desarrollo
- **Puerto expuesto:** `3000/tcp`
- **Comando de inicio:** `bun run dev` (arranca automáticamente al crear el sandbox)
- **Workdir:** `/app`
- **Env:** `NODE_ENV=development`
- **Arquitectura:** arm64

El sandbox expone el puerto `3000` del contenedor. Docker asigna automáticamente un puerto libre del host — ese puerto asignado se devuelve en la respuesta al crear el sandbox. La app Next.js queda accesible en `http://localhost:{puerto_asignado}`.

---

## Arquitectura del proyecto

```
/
├── main.go              # Servidor Gin, rutas /v1
├── sandbox/
│   └── client.go        # Wrapper del SDK de Docker (moby). Lógica de contenedores.
├── handlers/
│   ├── sandbox.go       # Handlers HTTP de todos los endpoints
│   └── errors.go        # Respuestas de error estandarizadas {code, message}
└── models/
    └── sandbox.go       # Structs de request/response
```

---

## Endpoints

Base URL: `http://localhost:8080/v1`

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/v1/sandboxes` | Listar sandboxes activos |
| `POST` | `/v1/sandboxes` | Crear e iniciar un sandbox |
| `GET` | `/v1/sandboxes/:id` | Inspeccionar un sandbox |
| `DELETE` | `/v1/sandboxes/:id` | Eliminar un sandbox (forzado) |
| `POST` | `/v1/sandboxes/:id/stop` | Detener un sandbox |
| `POST` | `/v1/sandboxes/:id/restart` | Reiniciar un sandbox |
| `POST` | `/v1/sandboxes/:id/exec` | Ejecutar un comando dentro del sandbox |

---

## Flujo de demo

### 1. Crear el sandbox

```bash
curl -X POST http://localhost:8080/v1/sandboxes \
  -H "Content-Type: application/json" \
  -d '{
    "image": "nextjs-docker:latest",
    "name": "my-nextjs",
    "ports": ["3000/tcp"]
  }'
```

```json
{
  "id": "a3f8c2d1e4b5...",
  "ports": {
    "3000/tcp": "32768"
  }
}
```

La app Next.js está disponible en `http://localhost:32768` (el puerto varía).

---

### 2. Listar sandboxes activos

```bash
curl http://localhost:8080/v1/sandboxes
```

```json
{
  "sandboxes": [
    {
      "Id": "a3f8c2d1...",
      "Names": ["/my-nextjs"],
      "Image": "nextjs-docker:latest",
      "State": "running",
      "Status": "Up 10 seconds"
    }
  ]
}
```

---

### 3. Inspeccionar el sandbox

```bash
curl http://localhost:8080/v1/sandboxes/a3f8c2d1
```

Devuelve el JSON completo de Docker con estado, puertos, variables de entorno, etc.

---

### 4. Ejecutar un comando dentro del sandbox

```bash
curl -X POST http://localhost:8080/v1/sandboxes/a3f8c2d1/exec \
  -H "Content-Type: application/json" \
  -d '{ "cmd": ["ls", "/app/src"] }'
```

```json
{ "output": "app\n" }
```

Otros comandos útiles para la demo:

```bash
# Ver archivos de la app
{ "cmd": ["ls", "-la", "/app/src/app"] }

# Ver el package.json
{ "cmd": ["cat", "/app/package.json"] }

# Ver versión de bun
{ "cmd": ["bun", "--version"] }
```

---

### 5. Detener el sandbox

```bash
curl -X POST http://localhost:8080/v1/sandboxes/a3f8c2d1/stop
```

```json
{ "status": "stopped" }
```

---

### 6. Reiniciar el sandbox

```bash
curl -X POST http://localhost:8080/v1/sandboxes/a3f8c2d1/restart
```

```json
{ "status": "restarted" }
```

---

### 7. Eliminar el sandbox

```bash
curl -X DELETE http://localhost:8080/v1/sandboxes/a3f8c2d1
```

`204 No Content`

---

## Errores

Todos los errores tienen la misma estructura:

```json
{
  "code": "NOT_FOUND",
  "message": "sandbox not found"
}
```

| HTTP | `code` | Cuándo ocurre |
|------|--------|---------------|
| `400` | `BAD_REQUEST` | Falta `image` u otro campo requerido |
| `404` | `NOT_FOUND` | El ID del sandbox no existe |
| `500` | `INTERNAL_ERROR` | Error del daemon Docker |

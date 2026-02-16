# Sandbox Orchestrator API

API REST para gestionar sandboxes (contenedores Docker). Corre en `:8080`.

Base URL: `http://localhost:8080/v1`

## Arrancar

```bash
go run main.go
```

---

## Rutas

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/v1/sandboxes` | Listar todos los sandboxes |
| `POST` | `/v1/sandboxes` | Crear e iniciar un sandbox |
| `GET` | `/v1/sandboxes/:id` | Obtener detalles de un sandbox |
| `DELETE` | `/v1/sandboxes/:id` | Eliminar un sandbox (forzado) |
| `POST` | `/v1/sandboxes/:id/stop` | Detener un sandbox |
| `POST` | `/v1/sandboxes/:id/restart` | Reiniciar un sandbox |
| `POST` | `/v1/sandboxes/:id/exec` | Ejecutar un comando dentro del sandbox |
| `GET` | `/v1/sandboxes/:id/files` | Leer un archivo del sandbox |
| `PUT` | `/v1/sandboxes/:id/files` | Escribir/crear un archivo en el sandbox |
| `DELETE` | `/v1/sandboxes/:id/files` | Eliminar un archivo o directorio del sandbox |
| `GET` | `/v1/sandboxes/:id/files/list` | Listar contenido de un directorio |

---

## Referencia de endpoints

### Listar sandboxes

```
GET /v1/sandboxes
```

Por defecto devuelve solo los sandboxes en ejecución. Con `?all=true` incluye los detenidos.

```bash
curl http://localhost:8080/v1/sandboxes
curl http://localhost:8080/v1/sandboxes?all=true
```

**Response `200`**
```json
{
  "sandboxes": [
    {
      "Id": "a3f8c2d1...",
      "Names": ["/my-nginx"],
      "Image": "nginx:latest",
      "State": "running",
      "Status": "Up 2 minutes"
    }
  ]
}
```

---

### Crear sandbox

```
POST /v1/sandboxes
```

Docker asigna automáticamente un puerto host disponible para cada puerto expuesto.

**Body**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `image` | string | si | Imagen Docker |
| `name` | string | no | Nombre del sandbox |
| `ports` | string[] | no | Puertos del contenedor a exponer |
| `env` | string[] | no | Variables de entorno |
| `cmd` | string[] | no | Override del CMD de la imagen |

```bash
curl -X POST http://localhost:8080/v1/sandboxes \
  -H "Content-Type: application/json" \
  -d '{
    "image": "nginx:latest",
    "name": "my-nginx",
    "ports": ["80/tcp"],
    "env": ["NGINX_HOST=localhost"]
  }'
```

**Response `201`**
```json
{
  "id": "a3f8c2d1e4b5...",
  "ports": {
    "80/tcp": "32768"
  }
}
```

---

### Obtener sandbox

```
GET /v1/sandboxes/:id
```

```bash
curl http://localhost:8080/v1/sandboxes/a3f8c2d1
```

**Response `200`**
```json
{
  "Id": "a3f8c2d1e4b5...",
  "Name": "/my-nginx",
  "State": {
    "Status": "running",
    "Running": true,
    "Pid": 1234
  },
  "NetworkSettings": {
    "Ports": {
      "80/tcp": [{ "HostIp": "0.0.0.0", "HostPort": "32768" }]
    }
  }
}
```

---

### Detener sandbox

```
POST /v1/sandboxes/:id/stop
```

```bash
curl -X POST http://localhost:8080/v1/sandboxes/a3f8c2d1/stop
```

**Response `200`**
```json
{ "status": "stopped" }
```

---

### Reiniciar sandbox

```
POST /v1/sandboxes/:id/restart
```

```bash
curl -X POST http://localhost:8080/v1/sandboxes/a3f8c2d1/restart
```

**Response `200`**
```json
{ "status": "restarted" }
```

---

### Eliminar sandbox

```
DELETE /v1/sandboxes/:id
```

Elimina el sandbox forzadamente aunque esté en ejecución.

```bash
curl -X DELETE http://localhost:8080/v1/sandboxes/a3f8c2d1
```

**Response `204 No Content`**

---

### Ejecutar comando en sandbox

```
POST /v1/sandboxes/:id/exec
```

**Body**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `cmd` | string[] | si | Comando y argumentos a ejecutar |

```bash
curl -X POST http://localhost:8080/v1/sandboxes/a3f8c2d1/exec \
  -H "Content-Type: application/json" \
  -d '{ "cmd": ["ls", "-la", "/usr/share/nginx/html"] }'
```

**Response `200`**
```json
{
  "output": "total 16\ndrwxr-xr-x 2 root root ...\n-rw-r--r-- 1 root root 497 index.html\n"
}
```

---

---

### Leer archivo

```
GET /v1/sandboxes/:id/files?path=<ruta>
```

```bash
curl "http://localhost:8080/v1/sandboxes/a3f8c2d1/files?path=/app/src/app/page.tsx"
```

**Response `200`**
```json
{
  "path": "/app/src/app/page.tsx",
  "content": "import Image from \"next/image\";\n..."
}
```

---

### Escribir archivo

```
PUT /v1/sandboxes/:id/files?path=<ruta>
```

Crea el archivo (y los directorios padre si no existen). Sobreescribe si ya existe.

**Body**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `content` | string | si | Contenido del archivo |

```bash
curl -X PUT "http://localhost:8080/v1/sandboxes/a3f8c2d1/files?path=/app/src/app/page.tsx" \
  -H "Content-Type: application/json" \
  -d '{ "content": "export default function Home() { return <h1>Hello</h1>; }" }'
```

**Response `200`**
```json
{ "path": "/app/src/app/page.tsx", "status": "written" }
```

---

### Eliminar archivo o directorio

```
DELETE /v1/sandboxes/:id/files?path=<ruta>
```

Elimina el archivo o directorio (recursivamente).

```bash
curl -X DELETE "http://localhost:8080/v1/sandboxes/a3f8c2d1/files?path=/app/src/app/old-file.tsx"
```

**Response `204 No Content`**

---

### Listar directorio

```
GET /v1/sandboxes/:id/files/list?path=<ruta>
```

`path` es opcional, por defecto `/`.

```bash
curl "http://localhost:8080/v1/sandboxes/a3f8c2d1/files/list?path=/app/src/app"
```

**Response `200`**
```json
{
  "path": "/app/src/app",
  "output": "total 16\n-rw-r--r-- 1 root root  523 page.tsx\n-rw-r--r-- 1 root root  312 layout.tsx\n"
}
```

---

## Errores

Todos los errores tienen la misma estructura:

```json
{
  "code": "NOT_FOUND",
  "message": "sandbox not found"
}
```

| HTTP | `code` | Descripción |
|------|--------|-------------|
| `400` | `BAD_REQUEST` | Body inválido o campo requerido faltante |
| `404` | `NOT_FOUND` | Sandbox o ruta no encontrada |
| `500` | `INTERNAL_ERROR` | Error del daemon Docker |

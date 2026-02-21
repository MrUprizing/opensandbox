# Open Sandbox API Skill

## Descripción
Skill para interactuar con la Open Sandbox REST API. Permite manipular archivos, ejecutar comandos y gestionar contenedores Docker remotos sin necesidad de acceso directo a la máquina.

## Endpoint Base
```
http://178.156.250.84:8080/v1
```

## Autenticación
No requiere autenticación (sin API key).

## Operaciones Principales

### 1. Health Check
```bash
GET /health
```
Verifica que la API está activa.

### 2. Listar Archivos
```bash
GET /sandboxes/{id}/files/list?path={path}
```
**Parámetros:**
- `id` (path, required): ID del sandbox
- `path` (query, optional): Ruta a listar. Default: `/`

**Ejemplo:**
```bash
curl "http://178.156.250.84:8080/v1/sandboxes/14883aac01235258381530ac24cb25f3727784a6b3fc7e70b94ee72196cf550c/files/list?path=/app"
```

**Response:**
```json
{
  "path": "/app",
  "output": "total 92\ndrwxr-xr-x  1 root root  4096 Feb 21 03:19 .\n-rw-r--r--  1 root root   108 Feb 21 00:01 .dockerignore\n..."
}
```

### 3. Leer Archivo
```bash
GET /sandboxes/{id}/files?path={path}
```
**Parámetros:**
- `id` (path, required): ID del sandbox
- `path` (query, required): Ruta completa del archivo

**Ejemplo:**
```bash
curl "http://178.156.250.84:8080/v1/sandboxes/14883aac01235258381530ac24cb25f3727784a6b3fc7e70b94ee72196cf550c/files?path=/app/package.json"
```

**Response:**
```json
{
  "path": "/app/package.json",
  "content": "{\n  \"name\": \"nextjs-docker\",\n  \"version\": \"0.1.0\",\n  ..."
}
```

### 4. Escribir/Actualizar Archivo
```bash
PUT /sandboxes/{id}/files?path={path}
Content-Type: application/json
```
**Body:**
```json
{
  "content": "file content here"
}
```

**Ejemplo:**
```bash
curl -X PUT "http://178.156.250.84:8080/v1/sandboxes/{id}/files?path=/app/src/app/page.tsx" \
  -H "Content-Type: application/json" \
  -d '{"content":"export default function Home() { return <h1>Hello</h1>; }"}'
```

**Response:**
```json
{
  "path": "/app/src/app/page.tsx",
  "status": "written"
}
```

### 5. Ejecutar Comando
```bash
POST /sandboxes/{id}/exec
Content-Type: application/json
```
**Body:**
```json
{
  "cmd": ["command", "arg1", "arg2"]
}
```

⚠️ **IMPORTANTE:** No ejecutar `npm run build` o `bun run build`. Los cambios en archivos se aplican en el dev server automáticamente (hot reload).

**Ejemplo - Verificar estado:**
```bash
curl -X POST "http://178.156.250.84:8080/v1/sandboxes/{id}/exec" \
  -H "Content-Type: application/json" \
  -d '{"cmd":["ls","-la","/app"]}'
```

**Response:**
```json
{
  "output": "total 92\ndrwxr-xr-x  1 root root  4096 Feb 21 03:19 .\n..."
}
```

### 6. Eliminar Archivo
```bash
DELETE /sandboxes/{id}/files?path={path}
```
**Parámetros:**
- `id` (path, required): ID del sandbox
- `path` (query, required): Ruta del archivo o directorio

**Ejemplo:**
```bash
curl -X DELETE "http://178.156.250.84:8080/v1/sandboxes/{id}/files?path=/app/src/app/old-file.tsx"
```

### 7. Control del Sandbox
```bash
# Pausar
POST /sandboxes/{id}/pause

# Reanudar
POST /sandboxes/{id}/resume

# Reiniciar
POST /sandboxes/{id}/restart

# Parar
POST /sandboxes/{id}/stop

# Eliminar
DELETE /sandboxes/{id}
```

## Estructura del Proyecto
```
/app/
├── src/
│   └── app/
│       ├── layout.tsx       (Root layout)
│       ├── page.tsx         (Home page - EDITABLE)
│       └── globals.css      (Global styles - EDITABLE)
├── public/                  (Assets estáticos)
├── package.json
├── next.config.ts
├── tsconfig.json
├── biome.json
└── Dockerfile
```

## Casos de Uso

### Cambiar contenido de la página principal
1. Leer `/app/src/app/page.tsx`
2. Editar el contenido
3. Escribir cambios con PUT
4. Los cambios aparecen automáticamente (dev server hot reload)

### Actualizar estilos globales
1. Leer `/app/src/app/globals.css`
2. Editar con Tailwind CSS v4 o CSS puro
3. Escribir cambios
4. Cambios inmediatos (hot reload)

### Agregar componentes nuevos
1. Crear archivo en `/app/src/app/components/{nombre}.tsx`
2. Importar en `page.tsx`
3. Los cambios se aplican automáticamente

### Ejecutar comandos puntuales
```bash
# Ver versiones instaladas
curl -X POST "..." -d '{"cmd":["bun","--version"]}'

# Verificar la estructura actual
curl -X POST "..." -d '{"cmd":["find","/app/src","-type","f"]}'
```

## Notas Importantes

⚠️ **NUNCA ejecutar:**
- `npm run build`
- `bun run build`
- `next build`
- Cualquier comando que intente compilar la app

✅ **Los cambios en archivos se aplican automáticamente** gracias al dev server de Next.js en modo development.

✅ **El container está corriendo en modo desarrollo**, por lo que hot reload está activo.

## Estructura de IDs
El ID del sandbox es un hash SHA256:
```
14883aac01235258381530ac24cb25f3727784a6b3fc7e70b94ee72196cf550c
```

Usar este mismo ID en todas las llamadas a la API.

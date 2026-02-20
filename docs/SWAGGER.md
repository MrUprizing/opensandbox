# Swagger Docs

API documentation is auto-generated from code annotations using [swag](https://github.com/swaggo/swag).

## Prerequisites

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

## Generate docs

```bash
swag init -g ./cmd/api/main.go -o docs --parseDependency --parseInternal
```

## Generate docs manually

```bash
/Users/uprizing/go/bin/swag init -g ./cmd/api/main.go -o docs --parseDependency --parseInternal
```

Run this every time you modify `@Summary`, `@Param`, `@Success`, etc. annotations in the handlers.

## Access

With the server running, open:

```
http://localhost:8080/swagger/index.html
```

## Files generated

| File | Purpose |
|------|---------|
| `docs/docs.go` | Embedded spec (imported by `main.go`) |
| `docs/swagger.json` | OpenAPI 2.0 spec (JSON) |
| `docs/swagger.yaml` | OpenAPI 2.0 spec (YAML) |

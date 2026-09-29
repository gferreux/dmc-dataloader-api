# dmc-dataloader-api

Go API project template for Dekuple services. Built on Clean Architecture (ports & adapters), dependency injection with Wire, and Viper-based configuration.

## Structure

```
cmd/dmc-dataloader-api/
  main.go         — entry point: flags, viper, slog, wire
  wire.go         — injection declarations (wireinject build tag)
  wire_gen.go     — wire-generated DI (committed)

internal/
  app/
    app.go                        — HTTP server, mux, routes
    handler/<domain>/
      handler.go                  — ServeHTTP, request parsing / response writing
      dto.go                      — request/response types

  domain/
    model/app_config.go           — configuration structs
    port/<domain>.go              — interfaces (usecases, daos, clients)
    port/mocks/                   — mockery-generated mocks
    usecase/<domain>/
      usecase.go                  — business logic implementation
      usecase_test.go

config-debug.yaml                 — local config (no secrets)
```

## Using this template

### 1. Copy and rename

```bash
cp -r dmc-dataloader-api dmc-my-service
cd dmc-my-service
```

Replace `dmc-dataloader-api` with the service name in:

- `go.mod` — module path
- `cmd/` — directory name
- `Dockerfile` — binary name
- `Makefile` — wire target
- `.mockery.yml` — package path
- `docker-compose.yaml` — appName

### 2. Add your domain

Rename/replace `helloworld` with your business domain:

```
internal/domain/port/mydomain.go          — define the interface
internal/domain/usecase/mydomain/         — implement the interface
internal/app/handler/mydomain/            — HTTP handler
```

Register the route in `internal/app/app.go`.

### 3. Update the config

Add the required fields to `internal/domain/model/app_config.go` and `config-debug.yaml`.

### 4. Regenerate Wire and mocks

```bash
make wire    # regenerates wire_gen.go after editing wire.go
make mocks   # regenerates internal/domain/port/mocks/
```

## Development

```bash
# Run locally
go run ./cmd/dmc-dataloader-api -config config-debug.yaml

# Tests
go test ./...

# Lint
golangci-lint run

# Docker
docker-compose up
```

## Environment variables

All config keys can be overridden via environment variables using the `DMC_` prefix, with `.` replaced by `_`.

```bash
DMC_SERVER_ADDR=:8080
DMC_LOG_LEVEL=debug
DMC_LOG_JSON=true
```

## CI/CD

| File | Trigger | Action |
|---|---|---|
| `.github/workflows/golang-lint.yml` | push | golangci-lint |
| `.github/workflows/golang-unit-test.yml` | push / workflow_dispatch | go test ./... |
| `cloudbuild.yaml` | Cloud Build trigger | build + push image |

Cloud Run deployment is commented out in `cloudbuild.yaml` — uncomment and set `_CLOUDRUN_SVC_NAME` or `_CLOUDRUN_JOB_NAME` as needed.

## Conventions

- **Ports** (interfaces) live in `internal/domain/port/` — never depend on adapters.
- **Usecases** only import `model` and `port` — never `net/http`, never a concrete DAO.
- **Handlers** contain no business logic — they translate HTTP ↔ domain.
- **Mocks** are generated, do not edit them manually.
- `wire_gen.go` is committed to allow building without the wire tool.

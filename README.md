# dmc-dataloader-api

HTTP API for the dmc-data-loader console. It creates, updates, and deletes `load_config` documents in Firestore. dmc-data-loader still owns ingestion: for each dropped file it walks `load_config` and uses the first active document whose `patterns.preprocess` or `patterns.ingest` regex matches `bucket/objectName`.

The contract shared with the Angular console is in [`api/openapi.yaml`](api/openapi.yaml).

## Structure

```
cmd/dmc-dataloader-api/
  main.go         — entry point: flags, viper, slog, wire
  wire.go         — injection declarations (wireinject build tag)
  wire_gen.go     — wire-generated DI (committed)

internal/
  app/            — HTTP server, middleware, handlers
  adapter/        — Firestore, in-memory repository, authenticators
  domain/
    model/        — LoadConfig and configuration structs
    port/         — usecase and repository interfaces
    catalog/      — CSV templates and /meta
    usecase/loadconfig/ — validation, overlap checks, filtering

api/openapi.yaml
config-debug.yaml — local config (no secrets)
```

## Conventions

- **Ports** (interfaces) live in `internal/domain/port/` — never depend on adapters.
- **Usecases** only import `model`, `port`, and `catalog` — never `net/http`, never a concrete DAO.
- **Handlers** contain no business logic — they translate HTTP ↔ domain.
- **Mocks** are generated, do not edit them manually.
- `wire_gen.go` is committed to allow building without the wire tool.

## Run locally

Against the Firestore emulator (does not touch the dev project):

```bash
gcloud emulators firestore start --host-port=localhost:8080
export FIRESTORE_EMULATOR_HOST=localhost:8080
export DMC_FIRESTORE_PROJECT_ID=demo-dmc
export DMC_FIRESTORE_DATABASE_ID=dmc-data-loader-dev
export DMC_FIRESTORE_COLLECTION=load_config
export DMC_AUTH_MODE=none
export DMC_SERVER_CORS_ORIGINS=http://localhost:4200
go run ./cmd/dmc-dataloader-api -config config-debug.yaml
```

`config-debug.yaml` points at the dev database. Environment variables override it, which is what keeps the emulator from writing to GCP. Application Default Credentials are used when `FIRESTORE_EMULATOR_HOST` is unset.

Docker Compose starts the emulator and the API together:

```bash
docker compose up --build
```

The API listens on `:3000` unless `PORT` is set (Cloud Run) or `DMC_SERVER_ADDR` is set.

```bash
go test ./...
golangci-lint run
make wire    # after editing wire.go
```

## Environment

All config keys use the `DMC_` prefix, with `.` replaced by `_`. A missing config file is allowed; defaults match the dev database below.

| Variable | Default |
|---|---|
| `DMC_FIRESTORE_PROJECT_ID` | `dmc-datastores-dev-becb` |
| `DMC_FIRESTORE_DATABASE_ID` | `dmc-data-loader-dev` |
| `DMC_FIRESTORE_COLLECTION` | `load_config` |
| `DMC_SERVER_ADDR` | `:3000` (`PORT` wins when this is unset) |
| `DMC_SERVER_CORS_ORIGINS` | empty (comma-separated list, or `*`) |
| `DMC_AUTH_MODE` | `none` |
| `DMC_AUTH_IAP_AUDIENCE` | empty |
| `DMC_LOG_LEVEL` | `info` |
| `DMC_LOG_JSON` | `true` when no config file is present |

Cloud Run sits behind Google IAP. Set `DMC_AUTH_MODE=iap` and `DMC_AUTH_IAP_AUDIENCE` to the IAP backend audience (`/projects/PROJECT_NUMBER/global/backendServices/SERVICE_ID`). The API verifies `X-Goog-IAP-JWT-Assertion` and logs that token's email on create, update, and delete. `DMC_AUTH_MODE=none` disables the check for local development. There is no second auth layer.

## What is stored

Documents stay compatible with the loader's decode:

- Field names are the Firestore names (`publisherName`, `bqParams`, `mappings.<column>.src`, …).
- `id`, `createTime`, and `updateTime` are document metadata, not fields.
- `partnerType` and `importType` are computed on read and tagged `firestore:"-"` so they are not written. They could not be confirmed as harmless extra fields because `github.com/dekuple-labs/dmc-domain` was not fetchable.

`partnerType` comes from `organization.type` when that value is `publisher` or `advertiser`, otherwise from `destination.datasetId`, otherwise from the import kind. `importType` prefers the last segment of the document id (`acme:demo:sales`) and otherwise uses `destination.tableId` (`profiles` is publisher opt-in).

## Mapping types

These integers are a working table, not a copy of `dmodel`. Confirm them before production writes.

| value | label | BigQuery type |
|---|---|---|
| 0 | STRING | STRING |
| 1 | INTEGER | INTEGER |
| 2 | FLOAT | FLOAT |
| 3 | BOOLEAN | BOOLEAN |
| 4 | TIMESTAMP | TIMESTAMP |
| 5 | DATE | DATE |
| 6 | JSON | JSON |

`GET /api/v1/meta` returns the same table. `POST /api/v1/load-configs/validate` reports errors (required fields, bad regex, unknown mode or type, mapping without `src`) and warnings (overlap of active patterns, unanchored regex, unescaped dots, incremental without `primaryKey`, mode/`incremental` mismatch, placeholder expressions such as `CONCAT("xxx","xxx")`, a delimiter stored as the two characters `\t`). Warnings do not block create or update.

Publisher opt-out and advertiser blacklists each use one required STRING column, `sha256_mobile_phone`. Customers and stores destination tables are assumed to be `customers` and `stores` in `dkp_dmc_advertisers_raw_eu_dev`.

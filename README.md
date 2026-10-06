# dmc-dataloader-api

HTTP API for the dmc-data-loader console. It creates, updates, and deletes `load_config` documents in Firestore. dmc-data-loader still owns ingestion: for each dropped file it walks `load_config` in document id order and uses the first document whose `patterns.preprocess` or `patterns.ingest` regex matches `bucket/objectName`.

The contract shared with the Angular console is in [`api/openapi.yaml`](api/openapi.yaml).

## Structure

```
cmd/dmc-dataloader-api/
  main.go         — entry point: flags, viper, slog, wire
  wire.go         — injection declarations (wireinject build tag)
  wire_gen.go     — wire-generated DI (committed)

internal/
  app/            — HTTP server, middleware, handlers
  adapter/        — Firestore, in-memory repository, authenticators, SFTPGo
  domain/
    model/        — LoadConfig and configuration structs
    port/         — usecase and repository interfaces
    catalog/      — CSV templates and /meta
    usecase/loadconfig/ — validation, overlap checks, derived identity

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

SFTP account routes stay disabled until `SFTPGO_URL` and `SFTPGO_API_KEY` are set. Export them in the shell before `docker compose up` when you want to call a local SFTPGo. Do not put the API key in `config-debug.yaml`.

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
| `SFTPGO_URL` | empty (SFTP routes other than config return 503) |
| `SFTPGO_API_KEY` | empty (Secret Manager on Cloud Run; never logged) |
| `DMC_SFTPGO_HOME_ROOT` | `/srv/sftpgo/data` |
| `DMC_LOG_LEVEL` | `info` |
| `DMC_LOG_JSON` | `true` when no config file is present |
| `DMC_ORGANIZATIONS_PROJECT_ID` | `dmc-datastores-dev-becb` |
| `DMC_ORGANIZATIONS_DATABASE_ID` | `(default)` |
| `DMC_ORGANIZATIONS_COLLECTION` | `organizations` |
| `DMC_ORGANIZATIONS_ACCOUNTS_COLLECTION` | `accounts` |
| `DMC_DERIVE_ADVERTISER_RAW_BUCKET` | `dkp-dmc-advertisers-raw-euw1-dev` |
| `DMC_DERIVE_ADVERTISER_STAGING_BUCKET` | `dkp-dmc-advertisers-staging-euw1-dev` |
| `DMC_DERIVE_ADVERTISER_NOTIFICATION_PROJECT_ID` | `dmc-curated-inventory-dev-e6da` |
| `DMC_DERIVE_ADVERTISER_NOTIFICATION_TOPIC_ID` | `dkp-dmc-data-loader-notifications-dev` |
| `DMC_DERIVE_ADVERTISER_DESTINATION_PROJECT_ID` | `dmc-raw-advertisers-dev-27c7` |
| `DMC_DERIVE_ADVERTISER_DESTINATION_DATASET_ID` | `dkp_dmc_advertisers_raw_eu_dev` |
| `DMC_DERIVE_PUBLISHER_RAW_BUCKET` | `dkp-dmc-publishers-raw-euw1-dev` |
| `DMC_DERIVE_PUBLISHER_STAGING_BUCKET` | `dkp-dmc-publishers-staging-euw1-dev` |
| `DMC_DERIVE_PUBLISHER_NOTIFICATION_PROJECT_ID` | `dmc-curated-inventory-dev-e6da` |
| `DMC_DERIVE_PUBLISHER_NOTIFICATION_TOPIC_ID` | `dkp-dmc-data-loader-notifications-dev` |
| `DMC_DERIVE_PUBLISHER_DESTINATION_PROJECT_ID` | `dmc-raw-publishers-dev-c69c` |
| `DMC_DERIVE_PUBLISHER_DESTINATION_DATASET_ID` | `dkp_dmc_publishers_raw_eu_dev` |
| `DMC_DERIVE_PUBLISHER_DESTINATION_TABLES_OPTIN` | `profiles` |
| `DMC_DERIVE_PUBLISHER_DESTINATION_TABLES_OPTOUT` | `optout` |

Cloud Run sits behind Google IAP. Set `DMC_AUTH_MODE=iap` and `DMC_AUTH_IAP_AUDIENCE` to the IAP backend audience (`/projects/PROJECT_NUMBER/global/backendServices/SERVICE_ID`). The API verifies `X-Goog-IAP-JWT-Assertion` and logs that token's email on create, update, and delete. `DMC_AUTH_MODE=none` disables the check for local development. There is no second auth layer.

## SFTP accounts

The console can create SFTPGo users for a publisher or advertiser client. Routes live under `/api/v1`, the same prefix as the rest of the API. This is the Go replacement for `create_sftp_client.py`. `referential` is not supported.

`GET /api/v1/sftp-accounts/config` tells the console whether SFTPGo is configured and which sub-folders and raw buckets apply. It returns 200 with `configured: false` when the SFTPGo env vars are missing. Publisher accounts get `optin`, `optout`, and `stop`. Advertiser accounts get `blacklists`, `customers`, `stores`, and `sales`. Each sub-folder is an SFTPGo virtual folder on that kind's raw bucket, the same buckets derive uses (`DMC_DERIVE_PUBLISHER_RAW_BUCKET`, `DMC_DERIVE_ADVERTISER_RAW_BUCKET`).

`SFTPGO_URL` is the SFTPGo server root, without an `/api/v2` suffix. `SFTPGO_API_KEY` is sent as `X-SFTPGO-API-KEY`. On Cloud Run the key belongs in Secret Manager and is mounted as the `SFTPGO_API_KEY` environment variable (`gcloud run services update --update-secrets=SFTPGO_API_KEY=<secret-name>:latest`). It is never written to logs. If either variable is missing, the process still starts and the other SFTP routes return 503. The rest of the API is unaffected.

`DMC_SFTPGO_HOME_ROOT` is the home directory prefix (default `/srv/sftpgo/data`). A new user's home is `<root>/<user>`.

`POST /api/v1/sftp-accounts/preview` is a dry run. `POST /api/v1/sftp-accounts` creates or extends the account. Password mode `generate` (the default) returns a 24-character password once in `generatedPassword`. Mode `none` requires `publicKeys`. An existing user's password is left unchanged. The audit log records the IAP email, the same way load_config create, update, and delete do.

## Derived identity

The console sends four inputs: `kind` (`advertiser` or `publisher`), `organizationName`, `nestedName` (an advertiser account or a publisher base), and `fileType`. `POST /api/v1/load-configs/derive` previews the result. Create uses those inputs plus `mode`, `bqParams`, and `mappings`. Any client value for a derived field is ignored.

`slug(x)` lowercases, trims, strips accents, turns spaces and `-` into `_`, and keeps `[a-z0-9_]`. An empty slug is rejected. The document id and `publisherName` are `{slug(organization)}:{slug(nested)}:{fileType}`, for example `bigmat_france:bigmat:sales`. Create returns 409 when that id exists.

Patterns are unanchored, matching the documents already in `load_config`. `TS` is `[0-9]{4}-[01][0-9]-[0-3][0-9]T[0-2][0-9]:[0-5][0-9]:[0-5][0-9]Z` and `EXT` is `[.](csv|zip|gz|gzip|tgz|tar.gz|7z)`. Derive does not warn about that shape. `POST /api/v1/load-configs/validate` still reports an unanchored regex or an unescaped dot when a document is checked as stored.

- Advertiser preprocess: `{ADV_RAW}/{org}/{nested}/{fileType}/.+EXT`
- Advertiser ingest: `{ADV_STAGING}/data/{TS}/{org}/{nested}/{fileType}/.+`
- Publisher preprocess: `{PUB_RAW}/{org}/{nested}/{fileType}/.+EXT`
- Publisher ingest: `{PUB_STAGING}/data/{TS}/{org}/{nested}/{fileType}/.+`

Advertiser `destination.tableId` is the file type. Publisher `optin` loads `profiles` and `optout` loads `optout`, both configurable. Notification and destination project/dataset are configurable per kind. Buckets and topics default to the dev values in the table above.

`organization.id` comes from an active document in the organizations collection whose slug and type match. An advertiser account is the active `accounts` document with that `organizationId` and a matching name slug; its id is `organization.account`. A publisher base has no id, so `organization.account` is `""`. If the directory does not contain the organization, or it cannot be read, the API reuses `organization.id` from an existing load_config of the same kind whose path contains that organization. An advertiser also needs the nested path segment so `organization.account` can be reused. A new publisher base can reuse the organization id from another base. Otherwise the API returns 422 naming the organization or account that was missing.

`GET /api/v1/organizations?type=advertiser|publisher` returns `[{id, name, slug}]`. `GET /api/v1/organizations/{id}/accounts` returns the same shape for advertiser accounts. `GET /api/v1/organizations/{slug}/bases?type=publisher` returns `[{name, slug}]` for base names already present in load_config paths. If the organization source is unreachable or not permitted, the organization and account lists fall back to load_config and a warning is logged.

PUT keeps stored derived values when the four inputs are omitted or unchanged, so a legacy pattern such as `ciblexo/.+` is not rewritten. Changing the organization, nested name, or file type re-derives the plumbing and moves the document to the new id. Unmodeled fields (`deactivated`, `incremental`, `isPartitionKey`) are still preserved. GET returns the derived fields. Documents whose id or patterns follow `org/nested/fileType` also return `kind`, `organizationName`, `nestedName`, and `fileType` parsed back from that convention. `kind` matches `partnerType` and `fileType` matches `importType`.

`FIRESTORE_EMULATOR_HOST` redirects both the load_config client and the organization client. Docker Compose points the organization source at the emulator as well.

## What is stored

The stored shape mirrors `dekuple-labs/dmc-domain/pkg/model/data_loader_config.go` (organization types from `pkg/model/organization_type.go`):

- Field names are the Firestore names (`publisherName`, `bqParams`, `mappings.<column>.src`, …).
- `bqParams.sourceFormat` is the integer `0` (CSV) or `1` (JSON).
- `organization.account` and `bqParams.nullMarker` are nullable.
- `id`, `createTime`, and `updateTime` are document metadata, not fields. They stand in for `DocumentHeader`.
- `partnerType`, `importType`, `kind`, `organizationName`, `nestedName`, and `fileType` are computed on read and tagged `firestore:"-"` so they are not written.
- Updates keep document fields the struct does not declare (`deactivated`, `incremental`, `mappings.<column>.isPartitionKey`, and any other extra). A mapping column left out of the body is removed.

`organization.type` on a new document is the wizard `kind`. List and read still return `referential` when a stored document has it, and an update that keeps that stored type is rejected. `partnerType` comes from `organization.type` when that value is `publisher` or `advertiser`, otherwise from `destination.datasetId`, otherwise from the import kind. `importType` prefers the last segment of the document id (`acme:demo:sales`) and otherwise uses `destination.tableId` (`profiles` is publisher opt-in).

## Mapping types

These are the domain iota values. `GET /api/v1/meta` returns the same rows. Template mappings that copy a CSV column use `0` (`RENAME`). The BigQuery column type on a template (`STRING`, `DATE`, …) is separate from this integer.

| value | label |
|---|---|
| 0 | RENAME |
| 1 | SQL |
| 2 | PREFIX_PATTERN |
| 3 | CUSTOM |
| 4 | EXTRA_FIELDS |
| 5 | MISSING_MAPPINGS |
| 6 | ARRAY |

`7` is ND and is rejected. `POST /api/v1/load-configs/validate` reports errors (required fields, bad regex, unknown mode or type, mapping without `src`, `organization.type` other than advertiser or publisher) and warnings (overlap with another config, unanchored regex, unescaped dots, `INCREMENTAL` without `primaryKey`, placeholder expressions such as `CONCAT("xxx","xxx")`, a delimiter stored as the two characters `\t`). Warnings do not block create or update. Overlap and test-pattern follow document id order, which is the loader's iteration order.

Publisher opt-out and advertiser blacklists each use one required STRING column, `sha256_mobile_phone`. Create sets an advertiser's `destination.tableId` to the file type (`blacklists`, `customers`, `stores`, or `sales`).


## Local Dev

# Config (CORS)
- `config-debug.yaml`:
```
server:
  addr: :3001
  read_timeout: 5s
  write_timeout: 15s
  shutdown_timeout: 10s
  cors_origins: http://localhost:3002,https://3002-ws-gferreux.cluster-ut4kyuiqi5d5suqbh6cuvr2cbg.cloudworkstations.dev

```

- `docker-compose.yaml`:
```
api:
    ...
    ports:
      - "3001:3001"
    environment:
      FIRESTORE_EMULATOR_HOST: firestore:8080
      DMC_FIRESTORE_PROJECT_ID: demo-dmc
      DMC_FIRESTORE_DATABASE_ID: dmc-data-loader-dev
      DMC_FIRESTORE_COLLECTION: load_config
      DMC_ORGANIZATIONS_PROJECT_ID: demo-dmc
      DMC_ORGANIZATIONS_DATABASE_ID: "(default)"
      DMC_ORGANIZATIONS_COLLECTION: organizations
      DMC_ORGANIZATIONS_ACCOUNTS_COLLECTION: accounts
      DMC_AUTH_MODE: none
      DMC_SERVER_CORS_ORIGINS: http://localhost:3002,https://3002-ws-gferreux.cluster-ut4kyuiqi5d5suqbh6cuvr2cbg.cloudworkstations.dev
      DMC_LOG_JSON: "false"
```


# Commands

- SFTPGO:
```
PROJECT="dmc-public-io-dev-d7ae"
INSTANCE=$(gcloud compute instances list --project "${PROJECT}" | tail -1 | awk '{print $1}')
ZONE=$(gcloud compute instances list --project "${PROJECT}" | tail -1 | awk '{print $2}')
gcloud compute ssh --zone "${ZONE}" "${INSTANCE}" --tunnel-through-iap --project "${PROJECT}" -- -NL 3003:localhost:8080
```

- API (+Firestore)
```
export SFTPGO_URL=localhost:3003
export SFTPGO_API_KEY="LeQMhufsKzJtHhMXzdow6J.QXHwSVM3hLAfVDzPpwv3oe"
docker build .
docker compose up
```

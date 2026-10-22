# Go implementation

This module provides a Go 1.27.3 implementation of the same fruit service exposed by the JVM implementations.

Request and response bodies use `encoding/json/v2`. The stricter v2 decoder rejects duplicate object names, invalid UTF-8, and trailing JSON, while the optimized Go 1.27 implementation replaces the previous hand-written response encoder.

## Requirements

- Go 1.27.3
- PostgreSQL on `localhost:5432` with database `fruits` and credentials `fruits` / `fruits`

You can start PostgreSQL from the repository root with:

```sh
cd run
docker compose up -d postgres
```

## Run

```sh
cd go
go run .
```

Or, with Task:

```sh
cd go
task run
```

To build a standalone binary:

```sh
cd go
task build
```

The server listens on `http://localhost:8080` by default and exposes:

- `GET /fruits?limit=50&offset=0`
- `GET /fruits/{name}`
- `POST /fruits`

Configuration is loaded from `app.yml` with `koanf`. Environment variables prefixed with `APP_` override YAML values. For example, `APP_SERVER_PORT=9090` overrides `server.port`.

List requests default to 50 fruits in ID order, accept limits from 1 to 100 and nonnegative offsets, and load prices only for that page. The pgx pool retains prepared statements and caches store metadata for 30 seconds. Creating a fruit does not invalidate unrelated store metadata. Requests have a five-second database deadline and POST bodies are limited to 64 KiB.

Go 1.27.3 and the direct module dependencies were checked against the upstream release registries on 2026-10-22 and are already current. The baseline binary explicitly disables PGO; the separate PGO build uses a profile trained with the same mixed workload as measurement, following [Go's PGO guidance](https://go.dev/doc/pgo).

## Tests

```sh
cd go
go test ./...
```

## Taskfile

The Go module includes a `Taskfile.yml` with a few common workflows:

- `task build` builds `server` or `server.exe`
- `task run` starts the service
- `task pgo-run-profile` starts the service with CPU profiling enabled and writes `server.pprof` when the process exits cleanly
- `task pgo-build` builds `server_pgo` or `server_pgo.exe` with `go build -pgo=server.pprof`
- `task pgo-run` builds and runs the optimized binary
- `task test` runs the test suite
- `task format` formats the source tree with `go fix` and `go fmt`
- `task tidy` runs `go mod tidy`

## PGO workflow

The PGO tasks are based on Go's CPU-profile-driven optimization flow:

1. Run `task pgo-run-profile`.
2. Exercise the service with representative traffic.
3. Stop the process so it flushes `server.pprof`.
4. Run `task pgo-build` to compile `server_pgo` or `server_pgo.exe` with that profile.
5. Run `task pgo-run` to launch the optimized binary.

The benchmark host automation performs that preparation during bootstrap: it starts PostgreSQL, runs a baseline binary with `-cpuprofile server.pprof`, drives representative load with `k6`, stops the profiled process to flush the profile, and builds `server_pgo`. The benchmark runner measures both `server` and `server_pgo`.

If you later want Go to pick up a profile automatically during plain `go build`, rename the chosen profile to `default.pgo` in the module root.

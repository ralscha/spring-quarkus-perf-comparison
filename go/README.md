# Go implementation

This module provides a Go 1.26.1 implementation of the same fruit service exposed by the Quarkus 3 application.

## Requirements

- Go 1.26.1
- PostgreSQL on `localhost:5432` with database `fruits` and credentials `fruits` / `fruits`

You can start PostgreSQL from the repository root with:

```sh
cd scripts
./infra.sh -s
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

- `GET /fruits`
- `GET /fruits/{name}`
- `POST /fruits`

Configuration is loaded from `app.yml` with `koanf`. Environment variables prefixed with `APP_` override YAML values. For example, `APP_SERVER_PORT=9090` overrides `server.port`.

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
- `task pgo-build` builds `server-pgo` or `server-pgo.exe` with `go build -pgo=server.pprof`
- `task pgo-run` builds and runs the optimized binary
- `task test` runs the test suite
- `task format` formats the source tree with `go fix` and `go fmt`
- `task tidy` runs `go mod tidy`

## PGO workflow

The PGO tasks are based on Go's CPU-profile-driven optimization flow:

1. Run `task pgo-run-profile`.
2. Exercise the service with representative traffic.
3. Stop the process so it flushes `server.pprof`.
4. Run `task pgo-build` to compile `server-pgo` or `server-pgo.exe` with that profile.
5. Run `task pgo-run` to launch the optimized binary.

If you later want Go to pick up a profile automatically during plain `go build`, rename the chosen profile to `default.pgo` in the module root.
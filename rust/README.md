# Rust / Axum

The same PostgreSQL-backed fruit API as the Java and Go services, implemented with
Axum, Tokio and deadpool-postgres. Rust 1.99.1 is pinned in `rust-toolchain.toml`;
`Cargo.lock` pins the resolved dependencies.

Initialize PostgreSQL with [`../run/db.sql`](../run/db.sql), then run:

```sh
cargo build --release --locked
./target/release/fruits-rust
```

The service listens on `0.0.0.0:8080` and connects to
`postgresql://fruits:fruits@localhost:5432/fruits`. It fails startup if PostgreSQL is
unavailable. Environment overrides:

| Variable | Default |
| --- | --- |
| `DATABASE_URL` | Uses the individual database settings below when unset |
| `APP_DATABASE_HOST` | `localhost` |
| `APP_DATABASE_PORT` | `5432` |
| `APP_DATABASE_NAME` | `fruits` |
| `APP_DATABASE_USER` | `fruits` |
| `APP_DATABASE_PASSWORD` | `fruits` |
| `APP_DATABASE_MAXCONNS` | `20` |
| `APP_SERVER_HOST` | `0.0.0.0` |
| `APP_SERVER_PORT` | `8080` |
| `APP_SERVER_MAXINFLIGHT` | `1024` |
| `RUST_LOG` | `info` |
| `TOKIO_WORKER_THREADS` | Available CPU count |

`DATABASE_URL` takes precedence over the individual database settings. PostgreSQL
connections use the benchmark's private network without TLS. TLS termination belongs
in the deployment configuration if this service is exposed beyond that network.

- `GET /fruits?limit=50&offset=0`: a stable page ordered by fruit ID, with prices
  ordered by store ID. `limit` must be 1–100 and `offset` nonnegative; invalid
  parameters return 400. Pagination is applied before joining prices.
- `GET /fruits/{name}`: case-sensitive lookup, 404 with an empty body when absent.
- `POST /fruits`: accepts a JSON name and optional description, returns 200 with
  the committed row, 400 for invalid input and 409 for an existing name. Names
  and descriptions are trimmed and limited to 255 Unicode characters. Client IDs
  and relationships are ignored, matching the other implementations.

Absent descriptions and empty `storePrices` are omitted. Reads perform one joined
query under PostgreSQL's statement snapshot and do not cache application data.
Each write uses one atomic INSERT in PostgreSQL's implicit transaction and the shared
`fruits_seq`, avoiding separate BEGIN/COMMIT round trips. The connection pool
caches prepared statements; its default 20-connection cap matches the other services.
Release builds use thin LTO and one code generation unit without machine-specific
CPU instructions, so benchmark binaries remain portable.
Accepted TCP connections use `TCP_NODELAY` to avoid delaying small API responses.

On Linux, each idle HTTP keep-alive connection also uses a file descriptor. The
benchmark runner sets both soft and hard open-file limits to 65,536 for all
implementations (`APP_NOFILE_LIMIT`). For a manual run, check `ulimit -Sn` and
`ulimit -Hn`; if the hard limit permits it, run `ulimit -Sn 65536` in the same shell
before starting the binary. A soft limit of 1,024 is too small for the benchmark's
2,000 preallocated VUs plus database connections and runtime descriptors.

Requests have a 64 KiB body limit and 5-second processing deadline. Admission is
bounded (503 when full), pool acquisition/connect timeouts are 3 seconds, and
PostgreSQL statement/lock timeouts are 5/3 seconds. Successful requests are not
logged individually. SIGINT/SIGTERM stop admission and drain in-flight requests.

```sh
cargo fmt --check
cargo clippy --locked --all-targets -- -D warnings
cargo test --locked
TEST_DATABASE_URL=postgresql://fruits:fruits@localhost:5432/fruits \
  cargo test --locked postgres_http_contract -- --ignored
```

The database test creates and drops an isolated schema, imports the shared seed,
and checks pagination, complete nested prices, lookups, SQL parameter binding,
create/read parity and recovery after a duplicate write. The database account needs
permission to create schemas. Ordinary tests do not require PostgreSQL.

Framework setup references: [Axum](https://docs.rs/axum/latest/axum/),
[Tokio](https://docs.rs/tokio/latest/tokio/),
[deadpool-postgres](https://docs.rs/deadpool-postgres/latest/deadpool_postgres/),
[Rust 1.99.1](https://blog.rust-lang.org/2026/10/15/Rust-1.99.1/).

# Spring, Quarkus, Micronaut, Go and Rust performance comparison

Five implementations serve the same PostgreSQL-backed fruit catalogue. The benchmark compares complete HTTP requests, including database access and JSON serialization.

| Project | Stable release checked on 2026-10-22 | Implementation |
| --- | --- | --- |
| [springboot4](springboot4/) | Spring Boot 4.1.2 | Spring MVC, Spring Data JPA, Hibernate, virtual threads |
| [quarkus3](quarkus3/) | Quarkus 3.40.1.1 | Quarkus REST and Hibernate ORM with Panache |
| [micronaut5](micronaut5/) | Micronaut Platform 5.2.3 | Micronaut HTTP, Hibernate/JPA, blocking work dispatched off the event loop |
| [go](go/) | Go 1.27.3, pgx 5.11.0 | Standard HTTP server, JSON v2 and pooled PostgreSQL connections |
| [rust](rust/) | Rust 1.99.1, Axum 0.8.9, Tokio 1.54.1 | Async HTTP and pooled PostgreSQL connections |

Framework BOMs manage their supported dependency combinations. Direct framework and runtime upgrades use stable releases; versions already current are retained. Transitive dependencies follow the stable framework or SDK that manages them. All Java projects target Java 27, and CI uses Java 27. The benchmark bootstrap installs the latest Temurin 27 patch for builds, Spring AOT-cache training and every JVM benchmark; the runner rejects any other Java major version. Maven wrappers use Maven 3.10.0.

Quarkus is consolidated into `quarkus3` using platform-thread dispatch. It produced the only fully passing run in the earlier local Java 25 comparison; workstation memory pressure and concurrent builds prevented a reliable repeated speed ranking. This selection is provisional, and those historical results do not establish a ranking for the Java 27 server benchmark. See the [Quarkus comparison notes](quarkus3/README.md) for the recorded evidence and limits.

Release references: [Spring Boot](https://github.com/spring-projects/spring-boot/releases), [Quarkus](https://github.com/quarkusio/quarkus/releases), [Micronaut Platform](https://github.com/micronaut-projects/micronaut-platform/releases), [Go](https://go.dev/doc/devel/release), [Rust](https://blog.rust-lang.org/2026/10/15/Rust-1.99.1/).

## Shared API and data

```text
GET  /fruits?limit=50&offset=0
GET  /fruits/{name}
POST /fruits
```

All services listen on port 8080 by default. Lists return a JSON array in fruit ID order; the default page size is 50, the maximum is 100, and offset defaults to zero. An unknown name returns 404. POST accepts a name and optional description and returns the persisted fruit. Responses include store prices and nested store/address data; empty values are omitted. Pagination happens in SQL before loading related data.

The shared [database seed](run/db.sql) contains 1,000 fruits, 100 stores and 10,000 prices. The first ten fruits retain their familiar names, including Apple and Pineapple; the rest use `Fruit0011` through `Fruit1000`. Each fruit has ten prices. Every implementation has a default connection pool limit of 20. Java modules retain small fixtures for their own tests; production benchmark runs use the shared seed.

PostgreSQL uses durable commits, WAL, checkpoints and autovacuum. The runner resets the dedicated benchmark database before each run and restarts the application, so writes and application caches from earlier runs cannot bias later ones.

## Run locally

From the repository root:

```sh
docker compose -f run/docker-compose.yml up -d --wait postgres
```

PostgreSQL listens on `127.0.0.1:5432`; database, user and password are `fruits`. The Compose volume persists data. Initial seed scripts run only on a new volume.

Build and start one implementation, from its project directory. For Java projects, set `JAVA_HOME` to your Java 27 JDK and put its `bin` directory first on `PATH`:

```sh
# Quarkus
./mvnw clean package
java -jar target/quarkus-app/quarkus-run.jar

# Spring Boot
./mvnw clean package
java -jar target/springboot4.jar

# Micronaut
./mvnw clean package
java -jar target/micronaut5.jar

# Go
go run .

# Rust
cargo build --release --locked
./target/release/fruits-rust
```

Use `mvnw.cmd` on Windows. Module READMEs describe configuration and optional optimization modes. Go's PGO variant is trained with representative traffic; Spring's AOT-cache variant uses a separate training run.

## Benchmark realistic traffic

With one service running and the shared seed initialized:

```sh
k6 run run/stress-k6.js
```

The workload uses open arrivals: offered traffic does not automatically fall when the service slows. It mixes 70% detail reads, 20% paginated lists, 5% expected misses and 5% creates, with an 80/20 hot-item distribution. The default profile includes a burst and recovery period; `K6_PROFILE=steady` selects a fixed arrival rate. Warmup exercises the same operation mix, including writes, in a separate scenario with distinct create names and a drain interval before measurement. Warmup failures invalidate the run while its latency and request counts stay outside measured scores. Keep-alive connections are reused, response content is sampled, and every write is checked.

Default SLOs are p95 below 300 ms, p99 below 1,000 ms, fewer than 1% failed HTTP requests, at least 99.9% successful checks and no dropped arrivals. Expected 404s are successful operations. Failed checks, dropped arrivals and incomplete runs are reported and must not be ranked as successful throughput. A write budget prevents accidental unbounded data growth. Local runs write `k6-result.json`.

The dedicated Linux runner adds multiple rates, repeated rounds, shuffled order, separate CPU sets for the service/database/load generator, memory limits, process resource sampling, database resets and machine-readable results. See the [runner guide](run/README.md) for installation, commands, settings and interpretation. Cloud provisioning remains optional through [Pulumi](run/pulumi/); it is not needed for local development.

These are controlled application benchmarks. TLS, a real network hop, authentication, production data distributions and external services need their own scenarios. Report hardware, settings, error rates, tail latency and repeat variation alongside throughput. Do not infer a universal framework ranking from a smoke test or a single run.

## Verification

Run each command from the corresponding project directory:

```sh
# go/
go test ./...
go vet ./...
# run/pulumi/
go test ./...
# rust/
cargo fmt --check
cargo clippy --locked --all-targets -- -D warnings
cargo test --locked
# Each Java project
./mvnw clean verify
```

Maven integration tests use Docker/Testcontainers. Go and Rust also provide PostgreSQL contract checks that create temporary schemas without resetting existing data:

```sh
# go/
FRUITS_TEST_DATABASE_URL=postgres://fruits:fruits@localhost:5432/fruits?sslmode=disable go test ./...
# rust/
TEST_DATABASE_URL=postgres://fruits:fruits@localhost:5432/fruits cargo test --locked postgres_http_contract -- --ignored
```

Run Maven projects sequentially when downloading dependencies to avoid contention on the shared cache. See [run/README.md](run/README.md) for runner regression checks.

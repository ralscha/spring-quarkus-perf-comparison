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

## Hetzner benchmark results (2026-10-25)

All five implementations passed every tested rate: **45 successful runs**, covering three repetitions at baseline rates of 100, 500 and 1,000 requests/second. The report records zero HTTP errors, failed checks and dropped arrivals. Go had the lowest p95 latency at every tested rate; all implementations sustained the highest tested load within the configured SLOs.

The measured checkout was `5c9c56cd870051403b58d85108a6c3b48a1d740a`. Sources: [JSON summary](report/summary.json), [TSV summary](report/summary.tsv), [host and settings](report/environment.txt), [execution schedule](report/schedule.tsv) and [PostgreSQL image metadata](report/postgres-image.json).

### Environment and workload

- **Host:** Hetzner `benchmark-hel1`, Debian 13, Linux 6.12.107, AMD EPYC-Milan under KVM, eight vCPUs (guest topology: four cores, two threads per core), approximately 30 GiB RAM visible to Linux, no swap.
- **Resource allocation:** application on CPUs 2-5 with a four-CPU quota and 6 GiB memory limit; PostgreSQL on CPUs 0-1 with a two-CPU quota and 2 GiB limit; k6 on CPUs 6-7. Requests used localhost on the same host.
- **Runtimes:** Temurin 27.0.1.0.2+1, Go 1.27.3, Rust 1.99.1, k6 2.3.0 and PostgreSQL 18.6. Only the five baseline implementations were measured (`BENCHMARK_VARIANTS=0`); Go PGO and Spring AOT-cache variants were excluded.
- **Procedure:** shuffled framework/rate order with seed 42, a fresh database and application per run, 60 seconds of warmup, a ten-second drain, then 120 seconds of measured mixed traffic. The operation mix was 70% detail reads, 20% paginated lists, 5% expected misses and 5% creates, with a burst to twice the baseline rate and recovery.

The burst makes the average offered rate 1.15 times the baseline: approximately 115, 575 and 1,150 requests/second across the three profiles. At the highest baseline, the burst peak was 2,000 requests/second. Achieved throughput below is completed measured requests divided by the full 120-second measurement window.

### Latency across tested rates

Each value is the **median of the three per-run p95 latencies**, in milliseconds, over the complete measured traffic mix.

| Implementation | 100 req/s baseline | 500 req/s baseline | 1,000 req/s baseline |
| --- | ---: | ---: | ---: |
| Go | 1.39 | 1.12 | 1.65 |
| Rust | 2.12 | 1.98 | 2.59 |
| Spring Boot | 4.00 | 3.03 | 3.44 |
| Micronaut | 3.87 | 3.03 | 3.47 |
| Quarkus | 3.79 | 3.09 | 3.57 |

### Highest tested baseline: 1,000 requests/second

Throughput and latency columns are medians across three runs, rounded for display. The p95 coefficient of variation (CV) measures variation between runs: sample standard deviation divided by the mean p95, expressed as a percentage.

| Implementation | Achieved req/s | p50 (ms) | p95 (ms) | p99 (ms) | p95 CV |
| --- | ---: | ---: | ---: | ---: | ---: |
| Go | 1,149.8 | 0.34 | 1.65 | 5.76 | 5.65% |
| Rust | 1,149.8 | 0.41 | 2.59 | 8.02 | 2.11% |
| Spring Boot | 1,149.8 | 0.54 | 3.44 | 8.70 | 1.30% |
| Micronaut | 1,149.8 | 0.53 | 3.47 | 8.62 | 0.82% |
| Quarkus | 1,149.8 | 0.51 | 3.57 | 9.07 | 1.30% |

Throughput was effectively equal because every implementation kept up with the offered traffic. At this load, the three JVM implementations had similar p95 latency, from 3.44 to 3.57 ms; all five implementations had median per-run p99 below 10 ms. Go's p95 CV was the largest at 5.65%, while the other implementations ranged from 0.82% to 2.11%.

The highest tested passing baseline was **1,000 requests/second for every implementation**. These runs did not establish maximum capacity; higher offered rates are needed to locate saturation. Results describe these complete implementations, including their SQL and cache strategies. The supplied report contains aggregates and environment metadata, without individual run logs or startup/CPU/memory measurements, so it does not support a resource-efficiency or startup-time comparison.

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

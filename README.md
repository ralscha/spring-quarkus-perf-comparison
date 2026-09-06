# Spring, Quarkus, Micronaut, and Go Performance Comparison

This repository compares several implementations of the same small fruit service:

- `springboot4`: Spring Boot 4, Spring MVC, Spring Data JPA, Hibernate ORM
- `quarkus3`: Quarkus 3, REST, Hibernate ORM with Panache
- `quarkus3-virtual`: Quarkus 3 with REST endpoints running on virtual threads
- `quarkus3-spring-compatibility`: Quarkus 3 using Spring Web/Data compatibility APIs
- `micronaut4`: Micronaut 4, Hibernate/JPA
- `micronaut5`: Micronaut 5, Hibernate/JPA
- `go`: Go HTTP service using `pgx`

All implementations expose the same benchmarked HTTP API on port `8080` by default:

```text
GET  /fruits
GET  /fruits/{name}
POST /fruits
```

The benchmark load script measures the read path only:

```text
GET /fruits
GET /fruits/Pineapple
GET /fruits/Apple
```

## Shared Data Model

Every implementation uses the same PostgreSQL schema and seed data:

- 10 fruits
- 8 stores
- 36 store/fruit price rows
- `Apple` and `Pineapple` are intentionally present because the k6 scenario reads them during every cycle

For fair comparisons, the examples are configured to keep these behaviors aligned:

- JSON omits empty values
- JDBC/connection pool size is 20
- Hibernate examples use second-level cache with JCache/Caffeine
- Hibernate examples use a batch fetch size of 16 where applicable
- Production runs use the shared PostgreSQL database instead of per-app seed imports

## Optimized Variants

The benchmark-host automation also exercises each runtime's current performance path:

- Go is measured as both a regular build and a workload-trained PGO build; it uses Go 1.27's `encoding/json/v2` API and faster allocator.
- Micronaut 4 and 5 are measured with fixed platform threads and with the Netty event-loop Loom carrier.
- Quarkus uses generated reflection-free REST/Jackson serializers, with a dedicated virtual-thread implementation.
- Spring Boot is measured in JVM, virtual-thread, and Spring AOT plus Java AOT-cache modes.

See the module READMEs for local commands and `run/cloud-init-benchmark-host.yaml` for the automated build, training, and launch configuration.

## Start PostgreSQL

From the repository root:

```sh
cd run
docker compose up -d postgres
```

The database listens on `localhost:5432` with database, username, and password all set to `fruits`.

## Run An Example

Build and run a JVM example from its directory:

```sh
cd quarkus3
./mvnw clean package
java -jar target/quarkus-app/quarkus-run.jar
```

For Spring Boot:

```sh
cd springboot4
./mvnw clean package
java -jar target/springboot4.jar
```

For Micronaut:

```sh
cd micronaut4
./mvnw clean package
java -jar target/micronaut4.jar
```

For Go:

```sh
cd go
go run .
```

## Run The Load Test

Start one implementation on `localhost:8080`, then run:

```sh
cd run
k6 run stress-k6.js
```

The load profile is controlled with environment variables:

- `K6_VUS`, default `100`
- `K6_WARMUP_RAMP_UP_SECONDS`, default `20`
- `K6_WARMUP_HOLD_SECONDS`, default `40`
- `K6_WARMUP_RAMP_DOWN_SECONDS`, default `10`
- `K6_MEASURED_HOLD_SECONDS`, default `20`
- `K6_GRACEFUL_RAMP_DOWN_SECONDS`, default `15`
- `K6_GRACEFUL_STOP_SECONDS`, default `15`

## Verification

Useful local checks:

```sh
cd go && go test ./...
cd run/pulumi && go test ./...
cd springboot4 && ./mvnw -q clean test
cd micronaut4 && ./mvnw -q clean test
cd micronaut5 && ./mvnw -q clean test
cd quarkus3 && ./mvnw -q clean test
cd quarkus3-spring-compatibility && ./mvnw -q clean test
cd quarkus3-virtual && ./mvnw -q clean test
```

Run Maven examples sequentially when they need to download fresh dependencies; parallel runs can contend on the shared local Maven cache.

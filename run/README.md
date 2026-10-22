# Running the benchmark

The runner compares the same paginated PostgreSQL API under a fixed offered request rate. It starts a fresh application and database for every framework, rate and repetition. Results retain failed runs; higher request counts alone do not establish a winner.

These are idiomatic implementations, so SQL shape, query counts and cache strategies can differ: Go caches store records, Java uses ORM caches, and Rust fetches joined rows. Fairness comes from the same API payloads, deterministic dataset and traffic, connection-pool ceiling, resource budgets, warmup and repeated measurements. Results describe these complete implementations, not an isolated measure of framework or language overhead.

## Local workload

Start the shared database, then start one application from its own directory:

```sh
docker compose -f run/docker-compose.yml up -d --wait
k6 run run/stress-k6.js
```

The Compose database contains 1,000 fruits, 100 stores and 10,000 prices. The first ten fruit names retain the small test fixture's names; the remaining names are `Fruit0011` through `Fruit1000`. Every seeded fruit has ten prices. The database uses normal WAL, synchronous commits, fsync, checkpoints and autovacuum. Data lives on a Docker volume, not a RAM filesystem.

The workload creates fruit records. To repeat a local run from the same initial data, stop the application and reset this disposable database before restarting:

```sh
docker compose -f run/docker-compose.yml down --volumes
docker compose -f run/docker-compose.yml up -d --wait
```

## Traffic and measurement

Each arrival performs one API request, with persistent HTTP connections and a five-second timeout. Requests use a deterministic sequence shared across implementations:

| Operation | Share | Behavior |
| --- | ---: | --- |
| Fruit detail | 70% | 80% of lookups target the first 100 fruits; the rest target the other 900 |
| Catalog page | 20% | 50 fruits, varying offsets from 0 through 950, ordered by ID |
| Missing fruit | 5% | An expected 404, counted as a successful response |
| Create fruit | 5% | Unique name, bounded description, committed database insert |

Warmup ramps for 15 seconds and holds for 45 seconds, exercising the same 70/20/5/5 mix, including committed creates, so the write path is warm before measurement. Create names include their phase to avoid collisions between warmup and measured iteration numbers. A ten-second drain separates the phases. Warmup and preflight samples are excluded from measured latency, request, error and dropped-arrival statistics. Warmup correctness, completion and dropped arrivals are reported separately; any warmup error or dropped arrival invalidates the run, without applying measured latency SLOs to cold startup. The standard measured profile lasts 120 seconds: 60 seconds at the baseline rate, six seconds ramping to twice that rate, twelve seconds at the peak, six seconds returning to baseline and 36 seconds of recovery. `steady` uses the baseline for the full measurement. Arrival-rate executors keep offering work when a server slows down, avoiding the self-throttling behavior of a fixed concurrency test. VUs are preallocated; insufficient capacity is reported as dropped iterations instead of silently reducing offered load.

Payload checks validate created names and IDs and sample read responses, including page order and length, all ten prices, unique stores and their complete addresses. Preflight verifies that the larger seed and pagination are active. The write budget covers warmup and measurement together and rejects configurations that could create more than 10,000 rows in one run; it never silently changes the workload mix. New rows from both phases are excluded from lookup/page selection, so the request distribution remains comparable while inserts still exercise real transactions and indexes. This benchmark does not model updating prices, browsing newly created fruit, authentication or third-party calls.

Every measured run reports p50, p95 and p99 latency, per-operation latency, completed requests, scheduled-window request rate, error/check failure rates and dropped iterations. The default SLO is p95 below 300 ms, p99 below 1,000 ms, HTTP failures below 1%, successful checks at least 99.9%, and zero dropped iterations. Incomplete or aborted runs fail. Request rate is completed requests divided by the scheduled measurement duration; requests finishing in the grace interval count toward their original measurement window.

## Dedicated host

`cloud-init-benchmark-host.yaml` clones the repository and invokes `bootstrap-benchmark-host.sh`. Set `repo_ref` to the reviewed commit being measured before provisioning; the default is `main`. Cloud-init does not include uncommitted workstation changes. The bootstrap builds all five implementations plus Go PGO and Spring AOT-cache artifacts. Go training uses the same workload separately from measurements. Java builds, AOT-cache training and all JVM benchmarks use the latest Temurin 27 patch installed in `/opt/java/openjdk-27`; Go 1.27.3, Rust 1.99.1, k6 2.3.0 and PostgreSQL 18.6 are pinned. Downloaded Java, Go and k6 archives are checksum-verified, and runtime/build metadata is retained.

The runner uses the absolute Java executable under `JAVA_HOME` for Micronaut, Quarkus, Spring Boot and Spring's AOT-cache variant. It rejects a missing executable or any Java major version other than 27 before starting the benchmark. Build and train the AOT cache with the same JDK installation used for measurement.

On an idle Debian 13 x86_64 host with at least eight CPUs:

```sh
sudo /usr/local/bin/run-benchmarks.sh
# Include independently labelled optimization builds:
sudo BENCHMARK_VARIANTS=1 /usr/local/bin/run-benchmarks.sh
# A smaller comparison:
sudo BENCHMARK_FRAMEWORKS=quarkus3,rust BENCHMARK_RATES=100,500 BENCHMARK_ROUNDS=3 \
  /usr/local/bin/run-benchmarks.sh
```

The default schedule includes `go`, `rust`, `micronaut5`, `quarkus3` and `springboot4`, three repetitions each at baseline rates 100, 500 and 1,000 requests/second. Framework/rate combinations are shuffled with a recorded seed within each repetition. The dedicated `fruits-benchmark` Compose project and its disposable volume are recreated before every run. The standard local Compose project is separate. A host lock prevents concurrent benchmark runners.

PostgreSQL receives CPUs 0–1 and 2 GiB RAM, each application CPUs 2–5 and a hard 6 GiB memory ceiling, and k6 CPUs 6–7. The runner checks CPU availability and overlap. Applications run in systemd cgroups with swap disabled and a four-CPU quota. JVM heap and Go's soft memory target are both 4 GiB; native runtimes share the same 6 GiB hard limit. Runtime thread counts reflect the application CPU allocation. No application runs concurrently with another measured application. Startup time, application CPU/memory samples, final peak memory and PostgreSQL statistics accompany each run. Check the host topology and resource logs before attributing differences to frameworks; a saturated database or load generator can limit every implementation.

The runner returns nonzero if any run fails a threshold or cannot finish, while continuing other runs after application/load-test failures. Fatal provisioning/database errors stop execution. Results go to a fresh `run/results/<UTC timestamp>/` directory: individual JSON and logs, the exact schedule, host/runtime metadata, `summary.tsv`, and `summary.json`. Aggregation reports medians, sample standard deviations and p95 coefficient of variation. Capacity is the **highest tested passing baseline rate**, requiring every repetition to pass; it is not an inferred maximum throughput. Compare latency at the same offered rate. Increase rates deliberately to locate saturation and repeat high-variance cases.

## Settings

All settings are environment variables. With `sudo`, set them after `sudo` as in the examples.

| Variable | Default | Meaning |
| --- | --- | --- |
| `JAVA_HOME` | `/opt/java/openjdk-27` | Java 27 JDK used by every JVM benchmark; other major versions are rejected |
| `K6_BASE_URL` | `http://127.0.0.1:8080` | Target for direct local k6 runs |
| `K6_RATE` | `100` | Baseline requests/second; set by the host runner per run |
| `K6_PROFILE` | `mixed` | `mixed` with burst/recovery, or `steady` |
| `K6_DURATION_SECONDS` | `120` | Measured duration, minimum 20 |
| `K6_BURST_MULTIPLIER` | `2` | Peak multiplier for mixed traffic |
| `K6_PREALLOCATED_VUS` | `max(100, rate × burst multiplier)` | Fixed VU allocation; tune to avoid load-generator saturation |
| `K6_WARMUP_RAMP_SECONDS` | `15` | Warmup ramp with the full operation mix |
| `K6_WARMUP_HOLD_SECONDS` | `45` | Warmup hold with the full operation mix |
| `K6_TIMEOUT_SECONDS` | `5` | Per-request timeout |
| `K6_GRACE_SECONDS` | `max(10, timeout + 1)` | Warmup drain and measured completion allowance |
| `K6_P95_MS`, `K6_P99_MS` | `300`, `1000` | Latency SLOs |
| `K6_ERROR_RATE` | `0.01` | Maximum HTTP error fraction, exclusive |
| `K6_MAX_CREATES` | `10000` | Safety bound on total warmup and measured writes |
| `K6_SEED` | `42` | Deterministic request sequence |
| `K6_RUN_ID` | Timestamp | Unique create-name prefix; host runner sets it |
| `K6_RESULT_FILE` | `k6-result.json` | Machine-readable summary path |
| `BENCHMARK_ROUNDS` | `3` | Repetitions per framework/rate |
| `BENCHMARK_RATES` | `100,500,1000` | Comma-separated baseline rates |
| `BENCHMARK_FRAMEWORKS` | `go,rust,micronaut5,quarkus3,springboot4` | Framework selection |
| `BENCHMARK_VARIANTS` | `0` | `1` adds Go PGO and Spring AOT-cache |
| `BENCHMARK_SEED` | `42` | Schedule shuffle seed and default workload seed |
| `RESULTS_DIR` | `run/results/<UTC timestamp>` | Must not contain an earlier schedule |
| `APP_CPUSET`, `APP_CPU_COUNT` | `2-5`, `4` | Application CPU allocation |
| `APP_MEMORY_LIMIT` | `6G` | Application cgroup memory ceiling |
| `POSTGRES_CPUSET`, `POSTGRES_CPU_LIMIT` | `0-1`, `2` | Dedicated database CPU allocation |
| `POSTGRES_MEMORY_LIMIT` | `2g` | Database memory ceiling |
| `K6_CPUSET` | `6-7` | Load generator CPU allocation |

## Verification

```sh
node --test run/workload.test.mjs
python3 -m unittest discover -s run -p 'test_*.py'
bash -n run/run-benchmarks.sh
bash -n run/bootstrap-benchmark-host.sh
docker compose -f run/docker-compose.yml config --quiet
```

CI also runs a short live k6 check against Go and PostgreSQL, with relaxed latency thresholds for shared CI hardware. This checks workload integration, not comparative performance.

Methodology references: [k6 arrival-rate allocation](https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/arrival-rate-vu-allocation/), [k6 dropped iterations](https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/dropped-iterations/), [PostgreSQL WAL reliability](https://www.postgresql.org/docs/18/wal-reliability.html), [k6 2.3.0](https://github.com/grafana/k6/releases/tag/v2.3.0), [PostgreSQL 18.6](https://www.postgresql.org/docs/release/18.6/).

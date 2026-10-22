Quarkus 3.40.1.1 is the latest stable release verified on 2026-10-22. Build and benchmark with Java 27; dependency versions follow the Quarkus platform BOM. Set `JAVA_HOME` and `PATH` to the same Java 27 JDK for the commands below.

The application uses Quarkus REST, build-time generated Jackson serializers, Hibernate ORM with Panache, and a 20-connection Agroal pool. `GET /fruits?limit=50&offset=0` selects bounded fruit IDs before fetching their complete prices and stores. Limits must be between 1 and 100; offsets must be nonnegative. Detail reads use a fetch join. Invalid pagination and oversized fields return 400; duplicate names return 409.

References: [3.40.1.1 release](https://quarkus.io/blog/quarkus-3-40-1-1-released/), [3.40 migration guide](https://github.com/quarkusio/quarkus/wiki/Migration-Guide-3.40), [virtual-thread guidance](https://quarkus.io/guides/virtual-threads/).

The retained implementation uses platform-thread dispatch. This is a **provisional selection**, based on the only completely passing historical Java 25 pilot: at 10 offered requests/second, platform dispatch had p95 61.27 ms and zero errors/drops; virtual dispatch had p95 373.91 ms and failed warmup. Earlier overloaded runs favored virtual dispatch but failed the benchmark thresholds. Paging and unrelated compilation on this Windows workstation prevented a reliable repeated fastest/capacity ranking. [The selection record](benchmark-selection.json) preserves every completed or setup-failed run, settings and limitations. These historical runs predate the Java 27 build target and do not establish a ranking for the current server configuration. Use the shared runner with Java 27 on an idle Linux host for a reliable capacity comparison.

## JVM Mode
To compile for JVM mode:
`./mvnw clean package`

To run in JVM Mode:
`java -jar target/quarkus-app/quarkus-run.jar`

## Native Mode
To compile for native mode:
`./mvnw clean package -Pnative`

To run in native mode:
`target/quarkus3-runner`

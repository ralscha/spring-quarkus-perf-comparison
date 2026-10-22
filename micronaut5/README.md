Micronaut Platform 5.2.3 is the latest stable platform verified against Maven Central on 2026-10-22. Build and benchmark with Java 27. Set `JAVA_HOME` and `PATH` to the same Java 27 JDK for the commands below. Framework libraries use the platform BOM; the Hibernate enhancer matches its managed Hibernate 7.4.10 runtime.

## JVM Mode
To compile for JVM mode:
`./mvnw clean package`

To run in JVM mode:
`java -jar target/micronaut5.jar`

Blocking JDBC endpoints use `@ExecuteOn(TaskExecutors.BLOCKING)`, which selects Micronaut's virtual-thread executor on Java 27. There is no fixed thread pool overriding it and no private JDK access required. Micronaut Serialization and dependency injection run their annotation processors at build time.

The shaded JAR preserves `Multi-Release: true`, so dependencies can select their optimized Java 21+ implementations instead of silently falling back to older bytecode.

`GET /fruits?limit=50&offset=0` accepts limits from 1 to 100 and nonnegative offsets. Database pagination selects a bounded set of fruit IDs, then fetches their complete prices and stores in one query. Detail reads use a fetch join. This avoids fetching the entire catalog or issuing a query for each related entity. Invalid pagination and oversized fields return 400; duplicate fruit names return 409.

Run `./mvnw clean verify` for the H2-backed HTTP and persistence tests. The shared benchmark uses PostgreSQL and `../run/db.sql`.

References: [Micronaut 5 baseline](https://micronaut.io/2026/05/20/micronaut-framework-5-0-0-released/), [platform BOM](https://repo.maven.apache.org/maven2/io/micronaut/platform/micronaut-platform/5.2.3/micronaut-platform-5.2.3.pom), [Hibernate fetching and pagination](https://docs.hibernate.org/stable/orm/querylanguage/html_single/).

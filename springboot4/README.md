Spring Boot 4.1.2 is the latest stable release verified against Maven Central on 2026-10-22. Build and benchmark with Java 27; dependency versions follow Spring Boot's BOM. Set `JAVA_HOME` and `PATH` to the same Java 27 JDK for the commands below.

Spring MVC uses virtual threads, HikariCP has a 20-connection limit, and Spring Boot 4.1's lazy connection fetching acquires connections only when SQL is needed. Open Session in View is disabled; DTO mapping happens inside service transactions. Hibernate enhancement runs during the build.

`GET /fruits?limit=50&offset=0` accepts limits from 1 to 100 and nonnegative offsets. Database pagination selects bounded fruit IDs before fetching their prices and stores. Detail reads fetch the same graph directly. Collection-fetch pagination fails instead of silently loading the entire catalog. Invalid pagination and oversized fields return 400; duplicate names return 409.

References: [Spring Boot releases](https://repo.maven.apache.org/maven2/org/springframework/boot/spring-boot-starter-parent/maven-metadata.xml), [virtual threads](https://docs.spring.io/spring-boot/reference/features/task-execution-and-scheduling.html), [lazy JDBC connections](https://docs.spring.io/spring-boot/reference/data/sql.html#data.sql.datasource.lazy-connection-proxy).

## JVM Mode
To compile for JVM mode:
`./mvnw clean package`

To run in JVM Mode:
`java -jar target/springboot4.jar`

## AOT on JVM mode
To compile for AOT mode:
`./mvnw clean compile spring-boot:process-aot package`

To run in AOT mode:
`java -Dspring.aot.enabled=true -jar target/springboot4.jar`

## AOT cache mode (Java 27)
After compiling for AOT mode, start PostgreSQL, extract the application, and train its JVM AOT cache:
`java -Djarmode=tools -jar target/springboot4.jar extract --destination target/springboot4-aot-cache`

From `target/springboot4-aot-cache`, train and run with the cache using the same Java 27 JDK installation:
`java -XX:AOTCacheOutput=app.aot -Dspring.aot.enabled=true -Dspring.context.exit=onRefresh -jar springboot4.jar`
`java -XX:AOTCache=app.aot -Dspring.aot.enabled=true -jar springboot4.jar`

## Native Mode
To compile for native mode:
`./mvnw clean native:compile -Pnative`

To run in native mode:
`target/springboot4`

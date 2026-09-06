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

## AOT cache mode (Java 25+)
After compiling for AOT mode, start PostgreSQL, extract the application, and train its JVM AOT cache:
`java -Djarmode=tools -jar target/springboot4.jar extract --destination target/springboot4-aot-cache`

From `target/springboot4-aot-cache`, train and run with the cache:
`java -XX:AOTCacheOutput=app.aot -Dspring.aot.enabled=true -Dspring.context.exit=onRefresh -jar springboot4.jar`
`java -XX:AOTCache=app.aot -Dspring.aot.enabled=true -jar springboot4.jar`

## Native Mode
To compile for native mode:
`./mvnw clean native:compile -Pnative`

To run in native mode:
`target/springboot4`

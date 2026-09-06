## JVM Mode
To compile for JVM mode:
`./mvnw clean package`

To run in JVM mode:
`java -jar target/micronaut5.jar`

## Virtual Thread Mode
To compile for virtual thread mode:
`./mvnw clean package`

To run in virtual thread mode with Micronaut's event-loop carrier:
`java --add-opens=java.base/java.lang=ALL-UNNAMED -Dmicronaut.executors.virtual.type=THREAD_PER_TASK -Dmicronaut.executors.virtual.virtual=true -Dmicronaut.server.thread-selection=blocking -Dmicronaut.netty.event-loops.default.loom-carrier=true -jar target/micronaut5.jar`

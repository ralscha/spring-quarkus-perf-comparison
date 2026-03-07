## JVM Mode
To compile for JVM mode:
`./mvnw clean package`

To run in JVM mode:
`java -jar target/micronaut4.jar`

## Virtual Thread Mode
To compile for virtual thread mode:
`./mvnw clean package`

To run in virtual thread mode:
`java -Dfruit.virtual-threads.enabled=true -jar target/micronaut4.jar`
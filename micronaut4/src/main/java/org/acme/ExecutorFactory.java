package org.acme;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

import io.micronaut.context.annotation.Bean;
import io.micronaut.context.annotation.Factory;
import io.micronaut.context.annotation.Property;
import jakarta.inject.Named;

@Factory
public class ExecutorFactory {
  @Bean(preDestroy = "close")
  @Named("fruit-executor")
  ExecutorService fruitExecutor(
      @Property(name = "fruit.executor.pool-size", defaultValue = "20") int poolSize,
      @Property(name = "fruit.virtual-threads.enabled", defaultValue = "false") boolean virtualThreads) {
    return virtualThreads ?
        Executors.newVirtualThreadPerTaskExecutor() :
        Executors.newFixedThreadPool(poolSize);
  }
}

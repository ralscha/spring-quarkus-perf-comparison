package org.acme;

import io.micronaut.core.annotation.Introspected;
import io.micronaut.runtime.Micronaut;
import jakarta.persistence.Entity;

@Introspected(packages = "org.acme.domain", includedAnnotations = Entity.class)
public final class Application {
  private Application() {
  }

  public static void main(String[] args) {
    Micronaut.run(Application.class, args);
  }
}

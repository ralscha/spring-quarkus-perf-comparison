package org.acme.repository;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.Optional;

import io.micronaut.test.extensions.junit5.annotation.MicronautTest;

import jakarta.inject.Inject;

import org.acme.domain.Fruit;
import org.junit.jupiter.api.Test;

@MicronautTest(transactional = false)
class FruitRepositoryTests {
  @Inject
  FruitRepository fruitRepository;

  @Test
  void pagesAreBoundedAndIncludeCompletePrices() {
    var page = fruitRepository.listPage(2, 1);
    assertThat(page).extracting(Fruit::getName).containsExactly("Pear", "Banana");
    assertThat(page.getFirst().getStorePrices()).hasSize(5);
    assertThat(fruitRepository.listPage(2, 10000)).isEmpty();
  }

  @Test
  void findByName() {
    this.fruitRepository.persist(new Fruit(null, "Grapefruit", "Summer fruit"));

    Optional<Fruit> fruit = this.fruitRepository.findByName("Grapefruit");
    assertThat(fruit)
        .isNotNull()
        .isPresent()
        .get()
        .extracting(Fruit::getName, Fruit::getDescription)
        .containsExactly("Grapefruit", "Summer fruit");

    assertThat(fruit.get().getId())
        .isNotNull()
        .isGreaterThan(2L);
  }
}

package org.acme.repository;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.Optional;

import jakarta.transaction.Transactional;

import org.acme.ContainersConfig;
import org.acme.domain.Fruit;
import org.junit.jupiter.api.Test;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.annotation.Import;

@SpringBootTest
@Transactional
@Import(ContainersConfig.class)
class FruitRepositoryTests {
	@Autowired
	FruitRepository fruitRepository;

  @Test
  void pagesAreBoundedAndIncludeCompletePrices() {
    var page = fruitRepository.listPage(2, 1);
    assertThat(page).extracting(Fruit::getName).containsExactly("Pear", "Banana");
    assertThat(page.getFirst().getStorePrices()).hasSize(5);
    assertThat(fruitRepository.listPage(2, 10000)).isEmpty();
  }

	@Test
	public void findByName() {
		this.fruitRepository.save(new Fruit(null, "Grapefruit", "Summer fruit"));

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

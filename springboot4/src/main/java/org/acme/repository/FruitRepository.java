package org.acme.repository;

import java.util.Optional;

import org.acme.domain.Fruit;

import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

public interface FruitRepository extends JpaRepository<Fruit, Long>, FruitPageRepository {
	@Query("select distinct f from Fruit f left join fetch f.storePrices p left join fetch p.store s where f.name = :name order by s.id")
	Optional<Fruit> findByName(String name);
}

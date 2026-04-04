package org.acme.repository;

import java.util.List;
import java.util.Optional;

import org.acme.domain.Fruit;

public interface FruitRepository {
  List<Fruit> listAll();

  Optional<Fruit> findByName(String name);

  void persist(Fruit fruit);
}
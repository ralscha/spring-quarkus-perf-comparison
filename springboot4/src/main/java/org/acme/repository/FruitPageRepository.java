package org.acme.repository;

import java.util.List;
import org.acme.domain.Fruit;

public interface FruitPageRepository {
  List<Fruit> listPage(int limit, int offset);
}

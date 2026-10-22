package org.acme.repository;

import java.util.List;
import jakarta.persistence.EntityManager;
import org.acme.domain.Fruit;

public class FruitPageRepositoryImpl implements FruitPageRepository {
  private final EntityManager entityManager;

  public FruitPageRepositoryImpl(EntityManager entityManager) {
    this.entityManager = entityManager;
  }

  @Override
  public List<Fruit> listPage(int limit, int offset) {
    // Page IDs first: paginating a collection fetch join loads the full table in memory.
    var ids = entityManager.createQuery("select f.id from Fruit f order by f.id", Long.class)
        .setFirstResult(offset).setMaxResults(limit).getResultList();
    if (ids.isEmpty()) {
      return List.of();
    }
    return entityManager.createQuery("""
        select distinct f from Fruit f
        left join fetch f.storePrices p left join fetch p.store s
        where f.id in :ids order by f.id, s.id
        """, Fruit.class).setParameter("ids", ids).getResultList();
  }
}

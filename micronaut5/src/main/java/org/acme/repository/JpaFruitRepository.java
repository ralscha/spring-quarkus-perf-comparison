package org.acme.repository;

import java.util.List;
import java.util.Optional;

import jakarta.inject.Singleton;
import jakarta.persistence.EntityManager;
import jakarta.transaction.Transactional;

import org.acme.domain.Fruit;

@Singleton
public class JpaFruitRepository implements FruitRepository {
  private final EntityManager entityManager;

  public JpaFruitRepository(EntityManager entityManager) {
    this.entityManager = entityManager;
  }

  @Override
  @Transactional(Transactional.TxType.SUPPORTS)
  public List<Fruit> listAll() {
    return this.entityManager.createQuery(
        """
        select distinct f
        from Fruit f
        left join fetch f.storePrices sfp
        left join fetch sfp.store s
        order by f.id, s.id
        """,
        Fruit.class)
        .getResultList();
  }

  @Override
  @Transactional(Transactional.TxType.SUPPORTS)
  public List<Fruit> listPage(int limit, int offset) {
    // Keep the SQL limit on IDs, never on a collection fetch join.
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

  @Override
  @Transactional(Transactional.TxType.SUPPORTS)
  public Optional<Fruit> findByName(String name) {
    return this.entityManager.createQuery(
        """
        select distinct f
        from Fruit f
        left join fetch f.storePrices sfp
        left join fetch sfp.store s
        where f.name = :name
        order by f.id, s.id
        """,
        Fruit.class)
        .setParameter("name", name)
        .getResultStream()
        .findFirst();
  }

  @Override
  @Transactional
  public void persist(Fruit fruit) {
    this.entityManager.persist(fruit);
    this.entityManager.flush();
  }
}

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
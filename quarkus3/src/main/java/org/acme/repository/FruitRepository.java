package org.acme.repository;

import java.util.Optional;
import java.util.List;

import jakarta.enterprise.context.ApplicationScoped;

import org.acme.domain.Fruit;

import io.quarkus.hibernate.orm.panache.PanacheRepository;

@ApplicationScoped
public class FruitRepository implements PanacheRepository<Fruit> {
	public Optional<Fruit> findByName(String name) {
		return find("select distinct f from Fruit f left join fetch f.storePrices p left join fetch p.store s where f.name = ?1 order by s.id", name).list().stream().findFirst();
	}

  public List<Fruit> listPage(int limit, int offset) {
    // Keep the SQL limit on IDs, never on a collection fetch join.
    var ids = getEntityManager().createQuery("select f.id from Fruit f order by f.id", Long.class)
        .setFirstResult(offset).setMaxResults(limit).getResultList();
    if (ids.isEmpty()) {
      return List.of();
    }
    return getEntityManager().createQuery("""
        select distinct f from Fruit f
        left join fetch f.storePrices p left join fetch p.store s
        where f.id in :ids order by f.id, s.id
        """, Fruit.class).setParameter("ids", ids).getResultList();
  }
}

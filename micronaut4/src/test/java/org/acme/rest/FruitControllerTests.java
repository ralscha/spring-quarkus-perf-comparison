package org.acme.rest;

import static io.restassured.RestAssured.given;
import static io.restassured.RestAssured.get;
import static org.hamcrest.Matchers.is;

import java.math.BigDecimal;
import java.util.List;
import java.util.Optional;

import io.micronaut.runtime.server.EmbeddedServer;
import io.micronaut.test.annotation.MockBean;
import io.micronaut.test.extensions.junit5.annotation.MicronautTest;

import jakarta.inject.Inject;

import org.acme.domain.Address;
import org.acme.domain.Fruit;
import org.acme.domain.Store;
import org.acme.domain.StoreFruitPrice;
import org.acme.repository.FruitRepository;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

import io.restassured.RestAssured;
import io.restassured.http.ContentType;

@MicronautTest
class FruitControllerTests {
  @Inject
  EmbeddedServer embeddedServer;

  @Inject
  FruitRepository fruitRepository;

  @BeforeEach
  void setUp() {
    RestAssured.port = this.embeddedServer.getPort();
    Mockito.reset(this.fruitRepository);
  }

  @MockBean(FruitRepository.class)
  FruitRepository fruitRepository() {
    return Mockito.mock(FruitRepository.class);
  }

  private static Fruit createFruit() {
    var price = BigDecimal.valueOf(1.29);
    var store = new Store(1L, "Some Store", new Address("123 Some St", "Some City", "USA"), "USD");
    var fruit = new Fruit(1L, "Apple", "Hearty Fruit");
    fruit.setStorePrices(List.of(new StoreFruitPrice(store, fruit, price)));
    return fruit;
  }

  @Test
  void getAll() {
    var fruit = createFruit();
    var fruitStorePrice = fruit.getStorePrices().getFirst();
    var store = fruitStorePrice.getStore();

    Mockito.when(this.fruitRepository.listAll())
        .thenReturn(List.of(fruit));

    get("/fruits").then()
        .statusCode(200)
        .contentType(ContentType.JSON)
        .body("$.size()", is(1))
        .body("[0].id", is(1))
        .body("[0].name", is("Apple"))
        .body("[0].description", is("Hearty Fruit"))
        .body("[0].storePrices[0].price", is(fruitStorePrice.getPrice().floatValue()))
        .body("[0].storePrices[0].store.name", is(store.getName()))
        .body("[0].storePrices[0].store.address.address", is(store.getAddress().address()))
        .body("[0].storePrices[0].store.address.city", is(store.getAddress().city()))
        .body("[0].storePrices[0].store.address.country", is(store.getAddress().country()))
        .body("[0].storePrices[0].store.currency", is(store.getCurrency()));

    Mockito.verify(this.fruitRepository).listAll();
    Mockito.verifyNoMoreInteractions(this.fruitRepository);
  }

  @Test
  void getFruitFound() {
    var fruit = createFruit();
    var fruitStorePrice = fruit.getStorePrices().getFirst();
    var store = fruitStorePrice.getStore();

    Mockito.when(this.fruitRepository.findByName("Apple"))
        .thenReturn(Optional.of(fruit));

    get("/fruits/Apple").then()
        .statusCode(200)
        .contentType(ContentType.JSON)
        .body("id", is(1))
        .body("name", is("Apple"))
        .body("description", is("Hearty Fruit"))
        .body("storePrices[0].price", is(fruitStorePrice.getPrice().floatValue()))
        .body("storePrices[0].store.name", is(store.getName()))
        .body("storePrices[0].store.address.address", is(store.getAddress().address()))
        .body("storePrices[0].store.address.city", is(store.getAddress().city()))
        .body("storePrices[0].store.address.country", is(store.getAddress().country()))
        .body("storePrices[0].store.currency", is(store.getCurrency()));

    Mockito.verify(this.fruitRepository).findByName("Apple");
    Mockito.verifyNoMoreInteractions(this.fruitRepository);
  }

  @Test
  void getFruitNotFound() {
    Mockito.when(this.fruitRepository.findByName("Apple"))
        .thenReturn(Optional.empty());

    get("/fruits/Apple").then()
        .statusCode(404);

    Mockito.verify(this.fruitRepository).findByName("Apple");
    Mockito.verifyNoMoreInteractions(this.fruitRepository);
  }

  @Test
  void addFruit() {
    Mockito.doAnswer(invocation -> {
      var fruit = invocation.getArgument(0, Fruit.class);
      fruit.setId(11L);
      return null;
    }).when(this.fruitRepository)
        .persist(Mockito.any(Fruit.class));

    given()
        .contentType(ContentType.JSON)
        .body("{\"name\":\"Grapefruit\",\"description\":\"Summer fruit\"}")
        .when().post("/fruits")
        .then()
        .contentType(ContentType.JSON)
        .statusCode(200)
        .body("id", is(11))
        .body("name", is("Grapefruit"))
        .body("description", is("Summer fruit"));

    Mockito.verify(this.fruitRepository).persist(Mockito.any(Fruit.class));
    Mockito.verifyNoMoreInteractions(this.fruitRepository);
  }
}
package org.acme.e2e;

import static io.restassured.RestAssured.get;
import static io.restassured.RestAssured.given;
import static org.hamcrest.Matchers.greaterThanOrEqualTo;
import static org.hamcrest.Matchers.is;

import java.math.BigDecimal;

import io.micronaut.runtime.server.EmbeddedServer;
import io.micronaut.test.extensions.junit5.annotation.MicronautTest;

import jakarta.inject.Inject;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.MethodOrderer.OrderAnnotation;
import org.junit.jupiter.api.Order;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestMethodOrder;

import io.restassured.RestAssured;
import io.restassured.http.ContentType;

@MicronautTest(transactional = false)
@TestMethodOrder(OrderAnnotation.class)
class FruitControllerEndToEndTest {
  private static final int DEFAULT_ORDER = 1;

  @Inject
  EmbeddedServer embeddedServer;

  @BeforeEach
  void setUp() {
    RestAssured.port = this.embeddedServer.getPort();
  }

  @Test
  @Order(DEFAULT_ORDER)
  void getAll() {
    get("/fruits").then()
        .statusCode(200)
        .contentType(ContentType.JSON)
        .body("$.size()", is(10))
        .body("[0].id", greaterThanOrEqualTo(1))
        .body("[0].name", is("Apple"))
        .body("[0].description", is("Hearty fruit"))
        .body("[0].storePrices[0].price", is(BigDecimal.valueOf(1.29).floatValue()))
        .body("[0].storePrices[0].store.name", is("Store 1"))
        .body("[0].storePrices[0].store.address.address", is("123 Main St"))
        .body("[0].storePrices[0].store.address.city", is("Anytown"))
        .body("[0].storePrices[0].store.address.country", is("USA"))
        .body("[0].storePrices[0].store.currency", is("USD"))
        .body("[0].storePrices[1].price", is(BigDecimal.valueOf(2.49).floatValue()))
        .body("[0].storePrices[1].store.name", is("Store 2"))
        .body("[0].storePrices[1].store.address.address", is("456 Main St"))
        .body("[0].storePrices[1].store.address.city", is("Paris"))
        .body("[0].storePrices[1].store.address.country", is("France"))
        .body("[0].storePrices[1].store.currency", is("EUR"))
        .body("[1].id", greaterThanOrEqualTo(1))
        .body("[1].name", is("Pear"))
        .body("[1].description", is("Juicy fruit"))
        .body("[2].name", is("Banana"))
        .body("[3].name", is("Orange"))
        .body("[4].name", is("Strawberry"))
        .body("[5].name", is("Mango"))
        .body("[6].name", is("Grape"))
        .body("[7].name", is("Pineapple"))
        .body("[8].name", is("Watermelon"))
        .body("[9].name", is("Kiwi"));
  }

  @Test
  @Order(DEFAULT_ORDER)
  void getFruitFound() {
    get("/fruits/Apple").then()
        .statusCode(200)
        .contentType(ContentType.JSON)
        .body("id", greaterThanOrEqualTo(1))
        .body("name", is("Apple"))
        .body("description", is("Hearty fruit"))
        .body("storePrices[0].price", is(BigDecimal.valueOf(1.29).floatValue()))
        .body("storePrices[0].store.name", is("Store 1"))
        .body("storePrices[1].price", is(BigDecimal.valueOf(2.49).floatValue()))
        .body("storePrices[1].store.name", is("Store 2"));
  }

  @Test
  @Order(DEFAULT_ORDER)
  void getFruitNotFound() {
    get("/fruits/XXXX").then()
        .statusCode(404);
  }

  @Test
  @Order(DEFAULT_ORDER + 1)
  void addFruit() {
    get("/fruits").then()
        .body("$.size()", is(10));

    given()
        .contentType(ContentType.JSON)
        .body("{\"name\":\"Another Lemon\",\"description\":\"Acidic fruit\"}")
        .when().post("/fruits")
        .then()
        .contentType(ContentType.JSON)
        .statusCode(200)
        .body("id", greaterThanOrEqualTo(3))
        .body("name", is("Another Lemon"))
        .body("description", is("Acidic fruit"));

    get("/fruits").then()
        .body("$.size()", is(11));
  }
}
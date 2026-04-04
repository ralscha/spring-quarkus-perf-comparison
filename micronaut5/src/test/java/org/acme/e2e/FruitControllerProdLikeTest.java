package org.acme.e2e;

import static io.restassured.RestAssured.get;
import static io.restassured.RestAssured.given;
import static org.hamcrest.Matchers.greaterThanOrEqualTo;
import static org.hamcrest.Matchers.is;

import io.micronaut.context.annotation.Property;
import io.micronaut.runtime.server.EmbeddedServer;
import io.micronaut.test.extensions.junit5.annotation.MicronautTest;

import jakarta.inject.Inject;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import io.restassured.RestAssured;
import io.restassured.http.ContentType;

@Property(name = "micronaut.environments", value = "prodlike")
@MicronautTest(environments = "prodlike", transactional = false)
class FruitControllerProdLikeTest {
  @Inject
  EmbeddedServer embeddedServer;

  @BeforeEach
  void setUp() {
    RestAssured.port = this.embeddedServer.getPort();
  }

  @Test
  void startsWithEmptyDatabase() {
    get("/fruits").then()
        .statusCode(200)
        .contentType(ContentType.JSON)
        .body("$.size()", is(0));

    given()
        .contentType(ContentType.JSON)
        .body("{\"name\":\"Prod Lemon\",\"description\":\"Prod fruit\"}")
        .when().post("/fruits")
        .then()
        .statusCode(200)
        .contentType(ContentType.JSON)
        .body("id", greaterThanOrEqualTo(1))
        .body("name", is("Prod Lemon"));

    get("/fruits").then()
        .body("$.size()", is(1));
  }
}
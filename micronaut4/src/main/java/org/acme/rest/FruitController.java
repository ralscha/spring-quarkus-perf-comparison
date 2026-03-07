package org.acme.rest;

import java.util.List;

import io.micronaut.http.HttpResponse;
import io.micronaut.http.annotation.Body;
import io.micronaut.http.annotation.Controller;
import io.micronaut.http.annotation.Get;
import io.micronaut.http.annotation.Post;
import io.micronaut.scheduling.annotation.ExecuteOn;

import jakarta.validation.Valid;

import org.acme.dto.FruitDTO;
import org.acme.service.FruitService;

@ExecuteOn("fruit-executor")
@Controller("/fruits")
public class FruitController {
  private final FruitService fruitService;

  public FruitController(FruitService fruitService) {
    this.fruitService = fruitService;
  }

  @Get
  public List<FruitDTO> getAll() {
    return this.fruitService.getAllFruits();
  }

  @Get("/{name}")
  public HttpResponse<FruitDTO> getFruit(String name) {
    return this.fruitService.getFruitByName(name)
        .map(HttpResponse::ok)
        .orElseGet(HttpResponse::notFound);
  }

  @Post
  public FruitDTO addFruit(@Body @Valid FruitDTO fruit) {
    return this.fruitService.createFruit(fruit);
  }
}
package org.acme.rest;

import java.util.List;

import io.micronaut.http.HttpResponse;
import io.micronaut.http.annotation.Body;
import io.micronaut.http.annotation.Controller;
import io.micronaut.http.annotation.Get;
import io.micronaut.http.annotation.Post;
import io.micronaut.http.annotation.QueryValue;
import io.micronaut.validation.Validated;
import io.micronaut.scheduling.TaskExecutors;
import io.micronaut.scheduling.annotation.ExecuteOn;

import jakarta.validation.Valid;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.Max;

import org.acme.dto.FruitDTO;
import org.acme.service.FruitService;

@ExecuteOn(TaskExecutors.BLOCKING)
@Controller("/fruits")
@Validated
public class FruitController {
  private final FruitService fruitService;

  public FruitController(FruitService fruitService) {
    this.fruitService = fruitService;
  }

  @Get
  public List<FruitDTO> getAll(
      @QueryValue(defaultValue = "50") @Min(1) @Max(100) int limit,
      @QueryValue(defaultValue = "0") @Min(0) int offset) {
    return this.fruitService.getAllFruits(limit, offset);
  }

  @Get("/{name}")
  public HttpResponse<FruitDTO> getFruit(String name) {
    return this.fruitService.getFruitByName(name)
        .map(HttpResponse::ok)
        .orElseGet(HttpResponse::notFound);
  }

  @Post
  public FruitDTO addFruit(@Body @Valid FruitDTO fruit) {
    try {
      return this.fruitService.createFruit(fruit);
    } catch (RuntimeException exception) {
      for (Throwable cause = exception; cause != null; cause = cause.getCause()) {
        if (cause instanceof java.sql.SQLException sql && "23505".equals(sql.getSQLState())) {
          throw new io.micronaut.http.exceptions.HttpStatusException(
              io.micronaut.http.HttpStatus.CONFLICT, "Fruit already exists");
        }
      }
      throw exception;
    }
  }
}

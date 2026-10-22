package org.acme.rest;

import java.util.List;

import jakarta.validation.Valid;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.Max;

import org.acme.dto.FruitDTO;
import org.acme.service.FruitService;

import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/fruits")
public class FruitController {
  private final FruitService fruitService;

  public FruitController(FruitService fruitService) {
    this.fruitService = fruitService;
  }

  @GetMapping(produces = MediaType.APPLICATION_JSON_VALUE)
  public List<FruitDTO> getAll(
      @RequestParam(defaultValue = "50") @Min(1) @Max(100) int limit,
      @RequestParam(defaultValue = "0") @Min(0) int offset) {
    return this.fruitService.getAllFruits(limit, offset);
  }

  @GetMapping(path = "/{name}", produces = MediaType.APPLICATION_JSON_VALUE)
  public ResponseEntity<FruitDTO> getFruit(@PathVariable String name) {
    return this.fruitService.getFruitByName(name)
      .map(ResponseEntity::ok)
      .orElseGet(() -> ResponseEntity.notFound().build());
  }

  @PostMapping(produces = MediaType.APPLICATION_JSON_VALUE, consumes = MediaType.APPLICATION_JSON_VALUE)
  public FruitDTO addFruit(@Valid @RequestBody FruitDTO fruit) {
    try {
      return this.fruitService.createFruit(fruit);
    } catch (RuntimeException exception) {
      for (Throwable cause = exception; cause != null; cause = cause.getCause()) {
        if (cause instanceof java.sql.SQLException sql && "23505".equals(sql.getSQLState())) {
          throw new org.springframework.web.server.ResponseStatusException(
              org.springframework.http.HttpStatus.CONFLICT, "Fruit already exists", exception);
        }
      }
      throw exception;
    }
  }
}

package org.acme.dto;

import java.util.ArrayList;
import java.util.List;

import io.micronaut.serde.annotation.Serdeable;

import jakarta.validation.constraints.NotBlank;

@Serdeable
public record FruitDTO(
    Long id,
    @NotBlank(message = "Name is mandatory") String name,
    String description,
    List<StoreFruitPriceDTO> storePrices
) {
  public FruitDTO {
    if (name == null) {
      throw new IllegalArgumentException("Name is mandatory");
    }

    if (storePrices == null) {
      storePrices = new ArrayList<>();
    }
  }
}
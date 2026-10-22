package org.acme.dto;

import java.util.ArrayList;
import java.util.List;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record FruitDTO(Long id, @NotBlank(message = "Name is mandatory") @Size(max = 255) String name, @Size(max = 255) String description, List<StoreFruitPriceDTO> storePrices) {
  public FruitDTO {
    if (name == null) {
      throw new IllegalArgumentException("Name is mandatory");
    }

    if (storePrices == null) {
      storePrices = new ArrayList<>();
    }
  }
}

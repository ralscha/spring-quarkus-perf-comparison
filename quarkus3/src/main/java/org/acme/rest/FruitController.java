package org.acme.rest;

import java.util.List;

import jakarta.validation.Valid;
import jakarta.ws.rs.BadRequestException;
import jakarta.ws.rs.DefaultValue;
import jakarta.ws.rs.QueryParam;
import jakarta.ws.rs.Consumes;
import jakarta.ws.rs.GET;
import jakarta.ws.rs.POST;
import jakarta.ws.rs.Path;
import jakarta.ws.rs.PathParam;
import jakarta.ws.rs.Produces;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;
import jakarta.ws.rs.core.Response.Status;

import org.acme.dto.FruitDTO;
import org.acme.service.FruitService;

@Path("/fruits")
public class FruitController {
	private final FruitService fruitService;

	public FruitController(FruitService fruitService) {
		this.fruitService = fruitService;
	}

	@GET
	@Produces(MediaType.APPLICATION_JSON)
	public List<FruitDTO> getAll(
      @QueryParam("limit") @DefaultValue("50") String limit,
      @QueryParam("offset") @DefaultValue("0") String offset) {
		return this.fruitService.getAllFruits(pageArgument(limit, 1, 100), pageArgument(offset, 0, Integer.MAX_VALUE));
	}

  private static int pageArgument(String value, int minimum, int maximum) {
    try {
      int parsed = Integer.parseInt(value);
      if (parsed >= minimum && parsed <= maximum) {
        return parsed;
      }
    } catch (NumberFormatException ignored) {
      // JAX-RS maps failed built-in parameter conversion to 404; our API uses 400.
    }
    throw new BadRequestException("Invalid pagination");
  }

	@GET
	@Path("/{name}")
	@Produces(MediaType.APPLICATION_JSON)
	public Response getFruit(@PathParam("name") String name) {
		return this.fruitService.getFruitByName(name)
			.map(fruit -> Response.ok(fruit).build())
			.orElseGet(() -> Response.status(Status.NOT_FOUND).build());
	}

	@POST
	@Produces(MediaType.APPLICATION_JSON)
	@Consumes(MediaType.APPLICATION_JSON)
	public FruitDTO addFruit(@Valid FruitDTO fruit) {
    try {
      return this.fruitService.createFruit(fruit);
    } catch (RuntimeException exception) {
      for (Throwable cause = exception; cause != null; cause = cause.getCause()) {
        if (cause instanceof java.sql.SQLException sql && "23505".equals(sql.getSQLState())) {
          throw new jakarta.ws.rs.WebApplicationException("Fruit already exists", exception, Status.CONFLICT);
        }
      }
      throw exception;
    }
	}
}

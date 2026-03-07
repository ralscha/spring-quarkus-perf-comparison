package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/quarkusio/spring-quarkus-perf-comparison/go/internal/fruit"
)

type Handler struct {
	logger     *slog.Logger
	repository fruit.Repository
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewHandler(logger *slog.Logger, repository fruit.Repository) http.Handler {
	handler := &Handler{logger: logger, repository: repository}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /fruits", handler.listFruits)
	mux.HandleFunc("GET /fruits/{name}", handler.getFruit)
	mux.HandleFunc("POST /fruits", handler.createFruit)

	return mux
}

func (h *Handler) listFruits(writer http.ResponseWriter, request *http.Request) {
	fruits, err := h.repository.ListFruits(request.Context())
	if err != nil {
		h.writeInternalError(writer, request, err)
		return
	}

	writeJSON(writer, http.StatusOK, fruits)
}

func (h *Handler) getFruit(writer http.ResponseWriter, request *http.Request) {
	fruitName := request.PathValue("name")
	fruitDTO, err := h.repository.GetFruitByName(request.Context(), fruitName)
	if err != nil {
		h.writeInternalError(writer, request, err)
		return
	}
	if fruitDTO == nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	writeJSON(writer, http.StatusOK, fruitDTO)
}

func (h *Handler) createFruit(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

	var payload fruit.FruitDTO
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(writer, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		writeJSON(writer, http.StatusBadRequest, errorResponse{Error: "name is mandatory"})
		return
	}

	created, err := h.repository.CreateFruit(request.Context(), payload)
	if err != nil {
		if errors.Is(err, fruit.ErrDuplicateFruit) {
			writeJSON(writer, http.StatusConflict, errorResponse{Error: err.Error()})
			return
		}
		h.writeInternalError(writer, request, err)
		return
	}

	writeJSON(writer, http.StatusOK, created)
}

func (h *Handler) writeInternalError(writer http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		h.logger.Info("request canceled", "method", request.Method, "path", request.URL.Path, "error", err)
		return
	}

	h.logger.Error("request failed", "method", request.Method, "path", request.URL.Path, "error", err)
	writeJSON(writer, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
}

func writeJSON(writer http.ResponseWriter, statusCode int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)

	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

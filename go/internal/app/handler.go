package app

import (
	"context"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

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

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		mux.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func (h *Handler) listFruits(writer http.ResponseWriter, request *http.Request) {
	limit, offset := 50, 0
	for name, target := range map[string]*int{"limit": &limit, "offset": &offset} {
		if values, ok := request.URL.Query()[name]; ok {
			if len(values) != 1 {
				writeJSON(writer, http.StatusBadRequest, errorResponse{Error: "invalid pagination"})
				return
			}
			value, err := strconv.Atoi(values[0])
			if err != nil || value < 0 || value > 2147483647 {
				writeJSON(writer, http.StatusBadRequest, errorResponse{Error: "invalid pagination"})
				return
			}
			*target = value
		}
	}
	if limit < 1 || limit > 100 {
		writeJSON(writer, http.StatusBadRequest, errorResponse{Error: "limit must be between 1 and 100"})
		return
	}
	fruits, err := h.repository.ListFruits(request.Context(), limit, offset)
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
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeJSON(writer, http.StatusUnsupportedMediaType, errorResponse{Error: "Content-Type must be application/json"})
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 64<<10)

	var payload fruit.FruitDTO
	if err := json.UnmarshalRead(request.Body, &payload); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeJSON(writer, http.StatusRequestEntityTooLarge, errorResponse{Error: "body exceeds 64 KiB"})
			return
		}
		writeJSON(writer, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" || len([]rune(payload.Name)) > 255 || len([]rune(payload.Description)) > 255 {
		writeJSON(writer, http.StatusBadRequest, errorResponse{Error: "name is required; name and description must be at most 255 characters"})
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
	if errors.Is(err, context.DeadlineExceeded) {
		writeJSON(writer, http.StatusGatewayTimeout, errorResponse{Error: "request timed out"})
		return
	}
	if errors.Is(err, context.Canceled) {
		h.logger.Info("request canceled", "method", request.Method, "path", request.URL.Path, "error", err)
		writer.WriteHeader(http.StatusRequestTimeout)
		return
	}

	h.logger.Error("request failed", "method", request.Method, "path", request.URL.Path, "error", err)
	writeJSON(writer, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
}

func writeJSON(writer http.ResponseWriter, statusCode int, payload any) {
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	writeJSONBytes(writer, statusCode, encodedPayload)
}

func writeJSONBytes(writer http.ResponseWriter, statusCode int, payload []byte) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)

	// Headers are already committed; a second error response would corrupt JSON.
	_, _ = writer.Write(payload)
}

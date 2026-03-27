package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quarkusio/spring-quarkus-perf-comparison/go/internal/fruit"
)

type stubRepository struct {
	listResult   []fruit.FruitDTO
	listErr      error
	listCalls    int
	getResult    *fruit.FruitDTO
	getErr       error
	getCalls     int
	createResult *fruit.FruitDTO
	createErr    error
	createdFruit fruit.FruitDTO
}

func (s *stubRepository) ListFruits(context.Context) ([]fruit.FruitDTO, error) {
	s.listCalls++
	return s.listResult, s.listErr
}

func (s *stubRepository) GetFruitByName(context.Context, string) (*fruit.FruitDTO, error) {
	s.getCalls++
	return s.getResult, s.getErr
}

func (s *stubRepository) CreateFruit(_ context.Context, item fruit.FruitDTO) (*fruit.FruitDTO, error) {
	s.createdFruit = item
	return s.createResult, s.createErr
}

func (s *stubRepository) Close() {}

func TestListFruits(t *testing.T) {
	repository := &stubRepository{
		listResult: []fruit.FruitDTO{{
			ID:          1,
			Name:        "Apple",
			Description: "Hearty fruit",
			StorePrices: []fruit.StoreFruitPriceDTO{{
				Price: 1.29,
				Store: &fruit.StoreDTO{
					ID:       1,
					Name:     "Store 1",
					Currency: "USD",
					Address: &fruit.AddressDTO{
						Address: "123 Main St",
						City:    "Anytown",
						Country: "USA",
					},
				},
			}},
		}},
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/fruits", nil)

	NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), repository).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var payload []fruit.FruitDTO
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(payload) != 1 || payload[0].Name != "Apple" || payload[0].StorePrices[0].Store.Name != "Store 1" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if repository.listCalls != 1 {
		t.Fatalf("expected repository to be called once, got %d", repository.listCalls)
	}
}

func TestGetFruitNotFound(t *testing.T) {
	repository := &stubRepository{}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/fruits/Apple", nil)

	NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), repository).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestCreateFruit(t *testing.T) {
	repository := &stubRepository{
		createResult: &fruit.FruitDTO{ID: 11, Name: "Grapefruit", Description: "Summer fruit"},
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/fruits", bytes.NewBufferString(`{"name":"Grapefruit","description":"Summer fruit"}`))
	request.Header.Set("Content-Type", "application/json")

	NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), repository).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if repository.createdFruit.Name != "Grapefruit" {
		t.Fatalf("expected created fruit name to be propagated, got %+v", repository.createdFruit)
	}
}

func TestCreateFruitValidation(t *testing.T) {
	repository := &stubRepository{}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/fruits", bytes.NewBufferString(`{"name":"   "}`))

	NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), repository).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestCreateFruitDuplicate(t *testing.T) {
	repository := &stubRepository{createErr: fruit.ErrDuplicateFruit}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/fruits", bytes.NewBufferString(`{"name":"Apple"}`))

	NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), repository).ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, response.Code)
	}
}

func TestListFruitsInternalError(t *testing.T) {
	repository := &stubRepository{listErr: errors.New("boom")}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/fruits", nil)

	NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), repository).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, response.Code)
	}
}

func TestListFruitsCanceledRequest(t *testing.T) {
	repository := &stubRepository{listErr: context.Canceled}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/fruits", nil)

	NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), repository).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("expected empty response body, got %q", response.Body.String())
	}
}

func TestListFruitsDeadlineExceeded(t *testing.T) {
	repository := &stubRepository{listErr: context.DeadlineExceeded}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/fruits", nil)

	NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), repository).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("expected empty response body, got %q", response.Body.String())
	}
}

package fruit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Each test run has its own schema, so no existing benchmark data is reset.
func TestPostgresRepository(t *testing.T) {
	url := os.Getenv("FRUITS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set FRUITS_TEST_DATABASE_URL to run PostgreSQL integration checks")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("go_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	// A single connection catches queries accidentally retaining earlier rows.
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	seed, err := os.ReadFile("../../../run/db.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(seed)); err != nil {
		t.Fatal(err)
	}
	repo := &PostgresRepository{pool: pool, storeCache: newStoreCache(defaultStoreCacheTTL)}

	page, err := repo.ListFruits(ctx, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].ID != 2 || page[1].ID != 3 || len(page[0].StorePrices) == 0 {
		t.Fatalf("unexpected page: %+v", page)
	}
	apple, err := repo.GetFruitByName(ctx, "Apple")
	if err != nil {
		t.Fatal(err)
	}
	if apple == nil || len(apple.StorePrices) == 0 || apple.StorePrices[0].Store.Address.Country == "" {
		t.Fatalf("missing nested prices or store: %+v", apple)
	}
	missing, err := repo.GetFruitByName(ctx, "' OR 1=1 --")
	if err != nil || missing != nil {
		t.Fatalf("missing lookup: %v %v", missing, err)
	}
	created, err := repo.CreateFruit(ctx, FruitDTO{Name: "Integration fruit", Description: "Written through repository"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID <= 10 {
		t.Fatalf("sequence overlaps fixture: %d", created.ID)
	}
	found, err := repo.GetFruitByName(ctx, created.Name)
	if err != nil || found == nil || found.ID != created.ID || len(found.StorePrices) != 0 {
		t.Fatalf("create/read round trip: %+v %v", found, err)
	}
	_, err = repo.CreateFruit(ctx, FruitDTO{Name: created.Name})
	if !errors.Is(err, ErrDuplicateFruit) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	page, err = repo.ListFruits(ctx, 50, 2147483647)
	if err != nil || len(page) != 0 {
		t.Fatalf("page beyond end: %+v %v", page, err)
	}
}

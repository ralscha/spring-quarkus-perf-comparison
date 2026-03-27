package fruit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quarkusio/spring-quarkus-perf-comparison/go/internal/config"
)

var ErrDuplicateFruit = errors.New("fruit already exists")

type Repository interface {
	ListFruits(ctx context.Context) ([]FruitDTO, error)
	GetFruitByName(ctx context.Context, name string) (*FruitDTO, error)
	CreateFruit(ctx context.Context, fruit FruitDTO) (*FruitDTO, error)
	Close()
}

type PostgresRepository struct {
	pool       *pgxpool.Pool
	storeCache *storeCache
}

func NewPostgresRepository(ctx context.Context, cfg config.DatabaseConfig) (*PostgresRepository, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.ConnString())
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	cfg.ApplyPoolConfig(poolConfig)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &PostgresRepository{
		pool:       pool,
		storeCache: newStoreCache(defaultStoreCacheTTL),
	}, nil
}

func (r *PostgresRepository) Pool() *pgxpool.Pool {
	return r.pool
}

func (r *PostgresRepository) Close() {
	r.pool.Close()
}

func (r *PostgresRepository) ListFruits(ctx context.Context) ([]FruitDTO, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, COALESCE(description, '')
		FROM fruits
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list fruits: %w", err)
	}
	defer rows.Close()

	fruits := make([]FruitDTO, 0, 16)
	fruitIDs := make([]int64, 0, 16)
	for rows.Next() {
		var fruit FruitDTO
		if err := rows.Scan(&fruit.ID, &fruit.Name, &fruit.Description); err != nil {
			return nil, fmt.Errorf("scan fruit: %w", err)
		}
		fruits = append(fruits, fruit)
		fruitIDs = append(fruitIDs, fruit.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fruits: %w", err)
	}

	if len(fruits) == 0 {
		return fruits, nil
	}

	pricesByFruit, err := r.loadStorePrices(ctx, fruitIDs)
	if err != nil {
		return nil, err
	}

	for index := range fruits {
		fruits[index].StorePrices = pricesByFruit[fruits[index].ID]
	}

	return fruits, nil
}

func (r *PostgresRepository) GetFruitByName(ctx context.Context, name string) (*FruitDTO, error) {
	var fruit FruitDTO
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(description, '')
		FROM fruits
		WHERE name = $1
	`, name).Scan(&fruit.ID, &fruit.Name, &fruit.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get fruit by name: %w", err)
	}

	pricesByFruit, err := r.loadStorePrices(ctx, []int64{fruit.ID})
	if err != nil {
		return nil, err
	}
	fruit.StorePrices = pricesByFruit[fruit.ID]

	return &fruit, nil
}

func (r *PostgresRepository) CreateFruit(ctx context.Context, fruit FruitDTO) (*FruitDTO, error) {
	fruit.Name = strings.TrimSpace(fruit.Name)
	fruit.Description = strings.TrimSpace(fruit.Description)

	created := FruitDTO{
		Name:        fruit.Name,
		Description: fruit.Description,
	}

	err := r.pool.QueryRow(ctx, `
		INSERT INTO fruits(name, description)
		VALUES ($1, NULLIF($2, ''))
		RETURNING id
	`, created.Name, created.Description).Scan(&created.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateFruit
		}
		return nil, fmt.Errorf("create fruit: %w", err)
	}

	r.storeCache.invalidateAll()

	return &created, nil
}

func (r *PostgresRepository) loadStorePrices(ctx context.Context, fruitIDs []int64) (map[int64][]StoreFruitPriceDTO, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			sfp.fruit_id,
			sfp.store_id,
			CAST(sfp.price AS DOUBLE PRECISION)
		FROM store_fruit_prices sfp
		WHERE sfp.fruit_id = ANY($1)
		ORDER BY sfp.fruit_id, sfp.store_id
	`, fruitIDs)
	if err != nil {
		return nil, fmt.Errorf("load store prices: %w", err)
	}
	defer rows.Close()

	pricesByFruit := make(map[int64][]StoreFruitPriceDTO, len(fruitIDs))
	missingStoreIDs := make([]int64, 0, 8)
	missingStoreSet := make(map[int64]struct{}, 8)
	for rows.Next() {
		var fruitID int64
		var storeID int64
		var price StoreFruitPriceDTO

		if err := rows.Scan(
			&fruitID,
			&storeID,
			&price.Price,
		); err != nil {
			return nil, fmt.Errorf("scan store price: %w", err)
		}

		cachedStore, ok := r.storeCache.get(storeID)
		if !ok {
			if _, seen := missingStoreSet[storeID]; !seen {
				missingStoreSet[storeID] = struct{}{}
				missingStoreIDs = append(missingStoreIDs, storeID)
			}
			cachedStore = &StoreDTO{ID: storeID}
		}
		price.Store = cachedStore

		pricesByFruit[fruitID] = append(pricesByFruit[fruitID], price)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate store prices: %w", err)
	}

	if len(missingStoreIDs) > 0 {
		storesByID, err := r.loadStoresByID(ctx, missingStoreIDs)
		if err != nil {
			return nil, err
		}
		r.storeCache.putMany(storesByID)
		for fruitID, prices := range pricesByFruit {
			for index := range prices {
				storeID := prices[index].Store.ID
				if store, ok := storesByID[storeID]; ok {
					prices[index].Store = cloneStore(store)
				}
			}
			pricesByFruit[fruitID] = prices
		}
	}

	return pricesByFruit, nil
}

func (r *PostgresRepository) loadStoresByID(ctx context.Context, storeIDs []int64) (map[int64]*StoreDTO, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, currency, address, city, country
		FROM stores
		WHERE id = ANY($1)
	`, storeIDs)
	if err != nil {
		return nil, fmt.Errorf("load stores by id: %w", err)
	}
	defer rows.Close()

	storesByID := make(map[int64]*StoreDTO, len(storeIDs))
	for rows.Next() {
		store := &StoreDTO{Address: &AddressDTO{}}
		if err := rows.Scan(
			&store.ID,
			&store.Name,
			&store.Currency,
			&store.Address.Address,
			&store.Address.City,
			&store.Address.Country,
		); err != nil {
			return nil, fmt.Errorf("scan store: %w", err)
		}
		storesByID[store.ID] = store
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stores: %w", err)
	}

	return storesByID, nil
}

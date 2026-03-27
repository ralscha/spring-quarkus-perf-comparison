package fruit

import (
	"testing"
	"time"
)

func TestStoreCacheReturnsClonedStore(t *testing.T) {
	cache := newStoreCache(defaultStoreCacheTTL)
	cache.putMany(map[int64]*StoreDTO{
		1: {
			ID:       1,
			Name:     "Store 1",
			Currency: "USD",
			Address: &AddressDTO{
				Address: "123 Main St",
				City:    "Anytown",
				Country: "USA",
			},
		},
	})

	store, ok := cache.get(1)
	if !ok {
		t.Fatal("expected cached store to be present")
	}
	store.Name = "Changed"
	store.Address.City = "Changed"

	storeAgain, ok := cache.get(1)
	if !ok {
		t.Fatal("expected cached store to still be present")
	}
	if storeAgain.Name != "Store 1" || storeAgain.Address.City != "Anytown" {
		t.Fatalf("expected cached store to be defensive copy, got %+v", storeAgain)
	}
}

func TestStoreCacheExpiresEntries(t *testing.T) {
	cache := newStoreCache(time.Millisecond)
	currentTime := time.Now()
	cache.nowFunc = func() time.Time { return currentTime }
	cache.putMany(map[int64]*StoreDTO{1: {ID: 1, Name: "Store 1"}})

	if _, ok := cache.get(1); !ok {
		t.Fatal("expected cached store to be present before expiry")
	}

	currentTime = currentTime.Add(2 * time.Millisecond)
	if _, ok := cache.get(1); ok {
		t.Fatal("expected cached store to expire")
	}
}

func TestStoreCacheInvalidatesAll(t *testing.T) {
	cache := newStoreCache(defaultStoreCacheTTL)
	cache.putMany(map[int64]*StoreDTO{1: {ID: 1, Name: "Store 1"}})
	cache.invalidateAll()

	if _, ok := cache.get(1); ok {
		t.Fatal("expected cache to be empty after invalidation")
	}
}
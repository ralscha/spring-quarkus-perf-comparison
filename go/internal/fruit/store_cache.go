package fruit

import (
	"sync"
	"time"
)

const defaultStoreCacheTTL = 30 * time.Second

type cachedStoreEntry struct {
	store     *StoreDTO
	expiresAt time.Time
}

type storeCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	byID    map[int64]cachedStoreEntry
	nowFunc func() time.Time
}

func newStoreCache(ttl time.Duration) *storeCache {
	if ttl <= 0 {
		ttl = defaultStoreCacheTTL
	}

	return &storeCache{
		ttl:     ttl,
		byID:    make(map[int64]cachedStoreEntry),
		nowFunc: time.Now,
	}
}

func (c *storeCache) get(id int64) (*StoreDTO, bool) {
	c.mu.RLock()
	entry, ok := c.byID[id]
	c.mu.RUnlock()
	if !ok || entry.store == nil || c.nowFunc().After(entry.expiresAt) {
		return nil, false
	}

	return cloneStore(entry.store), true
}

func (c *storeCache) putMany(stores map[int64]*StoreDTO) {
	if len(stores) == 0 {
		return
	}

	expiresAt := c.nowFunc().Add(c.ttl)

	c.mu.Lock()
	for id, store := range stores {
		if store == nil {
			continue
		}
		c.byID[id] = cachedStoreEntry{store: cloneStore(store), expiresAt: expiresAt}
	}
	c.mu.Unlock()
}

func (c *storeCache) invalidateAll() {
	c.mu.Lock()
	clear(c.byID)
	c.mu.Unlock()
}

func cloneStore(store *StoreDTO) *StoreDTO {
	if store == nil {
		return nil
	}

	cloned := &StoreDTO{
		ID:       store.ID,
		Name:     store.Name,
		Currency: store.Currency,
	}
	if store.Address != nil {
		cloned.Address = &AddressDTO{
			Address: store.Address.Address,
			City:    store.Address.City,
			Country: store.Address.Country,
		}
	}

	return cloned
}

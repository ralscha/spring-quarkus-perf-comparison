use std::{collections::HashMap, sync::RwLock, time::Duration};

use tokio::time::Instant;

use crate::Store;

/// Store rows change rarely, so reads reuse them for the same 30 seconds as the Go service.
pub(crate) const STORE_CACHE_TTL: Duration = Duration::from_secs(30);

#[derive(Debug)]
pub(crate) struct StoreCache {
    ttl: Duration,
    entries: RwLock<HashMap<i64, Entry>>,
}

#[derive(Debug)]
struct Entry {
    store: Store,
    expires_at: Instant,
}

impl StoreCache {
    pub(crate) fn new(ttl: Duration) -> Self {
        Self {
            ttl,
            entries: RwLock::new(HashMap::new()),
        }
    }

    pub(crate) fn get(&self, id: i64) -> Option<Store> {
        let entries = self
            .entries
            .read()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        let entry = entries.get(&id)?;
        if Instant::now() < entry.expires_at {
            Some(entry.store.clone())
        } else {
            None
        }
    }

    pub(crate) fn put(&self, stores: HashMap<i64, Store>) {
        if stores.is_empty() {
            return;
        }
        let expires_at = Instant::now() + self.ttl;
        let mut entries = self
            .entries
            .write()
            .unwrap_or_else(|poisoned| poisoned.into_inner());
        for (id, store) in stores {
            entries.insert(id, Entry { store, expires_at });
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::Address;

    fn store(id: i64) -> Store {
        Store {
            id,
            name: format!("Store {id}"),
            currency: "EUR".to_owned(),
            address: Address {
                address: "1 Market Street".to_owned(),
                city: "Zurich".to_owned(),
                country: "Switzerland".to_owned(),
            },
        }
    }

    #[tokio::test(start_paused = true)]
    async fn entries_expire_after_the_ttl() {
        let cache = StoreCache::new(STORE_CACHE_TTL);
        cache.put(HashMap::from([(1, store(1))]));

        assert!(cache.get(1).is_some());
        tokio::time::advance(STORE_CACHE_TTL).await;
        assert!(cache.get(1).is_none());
    }

    #[tokio::test(start_paused = true)]
    async fn unknown_ids_are_not_cached() {
        let cache = StoreCache::new(STORE_CACHE_TTL);
        cache.put(HashMap::from([(1, store(1))]));

        assert!(cache.get(2).is_none());
    }

    #[tokio::test(start_paused = true)]
    async fn reads_return_a_copy_of_the_cached_store() {
        let cache = StoreCache::new(STORE_CACHE_TTL);
        cache.put(HashMap::from([(1, store(1))]));

        let mut cached = cache.get(1).expect("store is cached");
        cached.name = "Changed".to_owned();
        cached.address.city = "Changed".to_owned();

        let cached = cache.get(1).expect("store is still cached");
        assert_eq!(cached.name, "Store 1");
        assert_eq!(cached.address.city, "Zurich");
    }
}

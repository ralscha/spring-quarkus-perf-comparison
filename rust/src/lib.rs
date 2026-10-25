pub mod config;
mod store_cache;

use std::{
    collections::{HashMap, HashSet},
    sync::Arc,
    time::Duration,
};

use axum::{
    Json, Router,
    extract::{DefaultBodyLimit, Path, Query, Request, State, rejection::JsonRejection},
    http::StatusCode,
    middleware::{self, Next},
    response::{IntoResponse, Response},
    routing::get,
};
use deadpool_postgres::{Object, Pool};
use serde::{Deserialize, Serialize};
use tokio::sync::Semaphore;
use tokio_postgres::error::SqlState;

use crate::store_cache::{STORE_CACHE_TTL, StoreCache};

// Pagination selects the fruit page first so the page limit never truncates prices.
const LIST_FRUITS: &str = "
    SELECT id, name, description
    FROM fruits
    ORDER BY id
    LIMIT $1 OFFSET $2";
// Prices are loaded for the page instead of joining the whole price table, and
// stores are resolved from the cache, so a page never re-reads store rows.
const LIST_PRICES: &str = "
    SELECT fruit_id, store_id, price::DOUBLE PRECISION
    FROM store_fruit_prices
    WHERE fruit_id = ANY($1)
    ORDER BY fruit_id, store_id";
const FIND_FRUIT: &str = "
    SELECT f.id, f.name, f.description, p.store_id, p.price::DOUBLE PRECISION
    FROM fruits f
    LEFT JOIN store_fruit_prices p ON p.fruit_id = f.id
    WHERE f.name = $1
    ORDER BY p.store_id";
const FIND_STORES: &str = "
    SELECT id, name, currency, address, city, country
    FROM stores
    WHERE id = ANY($1)";
const INSERT: &str = "
    INSERT INTO fruits (id, name, description)
    VALUES (nextval('fruits_seq'), $1, $2) RETURNING id";

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Fruit {
    pub id: i64,
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub store_prices: Vec<StorePrice>,
}

#[derive(Debug, Serialize)]
pub struct StorePrice {
    pub store: Store,
    pub price: f64,
}

#[derive(Clone, Debug, Serialize)]
pub struct Store {
    pub id: i64,
    pub name: String,
    pub currency: String,
    pub address: Address,
}

#[derive(Clone, Debug, Serialize)]
pub struct Address {
    pub address: String,
    pub city: String,
    pub country: String,
}

#[derive(Deserialize)]
struct CreateFruit {
    name: String,
    description: Option<String>,
}

impl CreateFruit {
    fn validate(mut self) -> Result<Self, ApiError> {
        self.name = self.name.trim().to_owned();
        if self.name.is_empty() || self.name.chars().count() > 255 {
            return Err(ApiError::bad_request(
                "name must contain 1 to 255 characters",
            ));
        }
        self.description = self
            .description
            .map(|value| value.trim().to_owned())
            .filter(|value| !value.is_empty());
        if self
            .description
            .as_ref()
            .is_some_and(|value| value.chars().count() > 255)
        {
            return Err(ApiError::bad_request(
                "description must contain at most 255 characters",
            ));
        }
        Ok(self)
    }
}

#[derive(Deserialize)]
struct Pagination {
    #[serde(default = "default_limit")]
    limit: i64,
    #[serde(default)]
    offset: i64,
}

const fn default_limit() -> i64 {
    50
}

#[derive(Clone)]
struct Limits {
    permits: Arc<Semaphore>,
    timeout: Duration,
}

#[derive(Clone)]
struct AppState {
    pool: Pool,
    stores: Arc<StoreCache>,
}

pub fn router(pool: Pool, max_in_flight: usize) -> Router {
    Router::new()
        .route("/fruits", get(list_fruits).post(create_fruit))
        .route("/fruits/{name}", get(find_fruit))
        .layer(DefaultBodyLimit::max(64 * 1024))
        .layer(middleware::from_fn_with_state(
            Limits {
                permits: Arc::new(Semaphore::new(max_in_flight)),
                timeout: Duration::from_secs(5),
            },
            bounded_request,
        ))
        .with_state(AppState {
            pool,
            stores: Arc::new(StoreCache::new(STORE_CACHE_TTL)),
        })
}

async fn bounded_request(State(limits): State<Limits>, request: Request, next: Next) -> Response {
    let Ok(_permit) = limits.permits.try_acquire() else {
        return ApiError(StatusCode::SERVICE_UNAVAILABLE, "server is busy").into_response();
    };
    match tokio::time::timeout(limits.timeout, next.run(request)).await {
        Ok(response) => response,
        Err(_) => ApiError(StatusCode::GATEWAY_TIMEOUT, "request timed out").into_response(),
    }
}

async fn list_fruits(
    State(state): State<AppState>,
    Query(page): Query<Pagination>,
) -> Result<Json<Vec<Fruit>>, ApiError> {
    if !(1..=100).contains(&page.limit) || page.offset < 0 {
        return Err(ApiError::bad_request(
            "limit must be 1 to 100 and offset must be nonnegative",
        ));
    }
    let client = state.pool.get().await?;
    let statement = client.prepare_cached(LIST_FRUITS).await?;
    let rows = client
        .query(&statement, &[&page.limit, &page.offset])
        .await?;
    let mut fruits = Vec::with_capacity(rows.len());
    for row in rows {
        fruits.push(Fruit {
            id: row.try_get(0)?,
            name: row.try_get(1)?,
            description: row
                .try_get::<_, Option<String>>(2)?
                .filter(|value| !value.is_empty()),
            store_prices: Vec::new(),
        });
    }
    if fruits.is_empty() {
        return Ok(Json(fruits));
    }
    let ids: Vec<i64> = fruits.iter().map(|fruit| fruit.id).collect();
    let statement = client.prepare_cached(LIST_PRICES).await?;
    let rows = client.query(&statement, &[&ids]).await?;
    let mut price_rows = Vec::with_capacity(rows.len());
    for row in rows {
        price_rows.push((row.try_get(0)?, row.try_get(1)?, row.try_get(2)?));
    }
    let mut prices = load_store_prices(&client, &state.stores, price_rows).await?;
    for fruit in &mut fruits {
        if let Some(store_prices) = prices.remove(&fruit.id) {
            fruit.store_prices = store_prices;
        }
    }
    Ok(Json(fruits))
}

async fn find_fruit(
    State(state): State<AppState>,
    Path(name): Path<String>,
) -> Result<Json<Fruit>, ApiError> {
    let client = state.pool.get().await?;
    let statement = client.prepare_cached(FIND_FRUIT).await?;
    let rows = client.query(&statement, &[&name]).await?;
    let Some(first) = rows.first() else {
        return Err(ApiError(StatusCode::NOT_FOUND, ""));
    };
    let id: i64 = first.try_get(0)?;
    let mut fruit = Fruit {
        id,
        name: first.try_get(1)?,
        description: first
            .try_get::<_, Option<String>>(2)?
            .filter(|value| !value.is_empty()),
        store_prices: Vec::new(),
    };
    let mut price_rows = Vec::with_capacity(rows.len());
    for row in &rows {
        let Some(store_id) = row.try_get::<_, Option<i64>>(3)? else {
            continue;
        };
        price_rows.push((id, store_id, row.try_get(4)?));
    }
    let mut prices = load_store_prices(&client, &state.stores, price_rows).await?;
    fruit.store_prices = prices.remove(&id).unwrap_or_default();
    Ok(Json(fruit))
}

async fn create_fruit(
    State(state): State<AppState>,
    payload: Result<Json<CreateFruit>, JsonRejection>,
) -> Result<Json<Fruit>, ApiError> {
    let Json(payload) = payload.map_err(|error| {
        let status = match error.status() {
            StatusCode::PAYLOAD_TOO_LARGE => StatusCode::PAYLOAD_TOO_LARGE,
            StatusCode::UNSUPPORTED_MEDIA_TYPE => StatusCode::UNSUPPORTED_MEDIA_TYPE,
            _ => StatusCode::BAD_REQUEST,
        };
        ApiError(status, "invalid JSON body")
    })?;
    let payload = payload.validate()?;
    let client = state.pool.get().await?;
    let statement = client.prepare_cached(INSERT).await?;
    // One INSERT is atomic in PostgreSQL's implicit transaction. Explicit BEGIN
    // and COMMIT would add two network round trips without changing this operation.
    let row = client
        .query_one(&statement, &[&payload.name, &payload.description])
        .await?;
    let id = row.try_get(0)?;
    Ok(Json(Fruit {
        id,
        name: payload.name,
        description: payload.description,
        store_prices: Vec::new(),
    }))
}

/// Resolves the store of every price row, querying only the stores the cache does not have.
async fn load_store_prices(
    client: &Object,
    cache: &StoreCache,
    rows: Vec<(i64, i64, f64)>,
) -> Result<HashMap<i64, Vec<StorePrice>>, ApiError> {
    let mut seen = HashSet::new();
    let mut stores = HashMap::new();
    let mut missing = Vec::new();
    for (_, store_id, _) in &rows {
        if !seen.insert(*store_id) {
            continue;
        }
        match cache.get(*store_id) {
            Some(store) => {
                stores.insert(*store_id, store);
            }
            None => missing.push(*store_id),
        }
    }

    if !missing.is_empty() {
        let statement = client.prepare_cached(FIND_STORES).await?;
        let loaded = client.query(&statement, &[&missing]).await?;
        let mut loaded_stores = HashMap::with_capacity(loaded.len());
        for row in loaded {
            let store = Store {
                id: row.try_get(0)?,
                name: row.try_get(1)?,
                currency: row.try_get(2)?,
                address: Address {
                    address: row.try_get(3)?,
                    city: row.try_get(4)?,
                    country: row.try_get(5)?,
                },
            };
            loaded_stores.insert(store.id, store);
        }
        for (id, store) in &loaded_stores {
            stores.insert(*id, store.clone());
        }
        cache.put(loaded_stores);
    }

    let mut prices: HashMap<i64, Vec<StorePrice>> = HashMap::new();
    for (fruit_id, store_id, price) in rows {
        // The foreign key guarantees a store row for every price.
        let store = stores.get(&store_id).ok_or(ApiError(
            StatusCode::INTERNAL_SERVER_ERROR,
            "store metadata is unavailable",
        ))?;
        prices.entry(fruit_id).or_default().push(StorePrice {
            store: store.clone(),
            price,
        });
    }
    Ok(prices)
}

#[derive(Debug)]
struct ApiError(StatusCode, &'static str);

impl ApiError {
    fn bad_request(message: &'static str) -> Self {
        Self(StatusCode::BAD_REQUEST, message)
    }
}

impl From<tokio_postgres::Error> for ApiError {
    fn from(error: tokio_postgres::Error) -> Self {
        match error.code() {
            Some(&SqlState::UNIQUE_VIOLATION) => Self(StatusCode::CONFLICT, "fruit already exists"),
            Some(&SqlState::QUERY_CANCELED | &SqlState::LOCK_NOT_AVAILABLE) => {
                Self(StatusCode::SERVICE_UNAVAILABLE, "database timed out")
            }
            _ => {
                tracing::error!(%error, "database request failed");
                Self(StatusCode::INTERNAL_SERVER_ERROR, "internal server error")
            }
        }
    }
}

impl From<deadpool_postgres::PoolError> for ApiError {
    fn from(error: deadpool_postgres::PoolError) -> Self {
        tracing::warn!(%error, "database connection unavailable");
        Self(StatusCode::SERVICE_UNAVAILABLE, "database unavailable")
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        if self.1.is_empty() {
            return self.0.into_response();
        }
        #[derive(Serialize)]
        struct ErrorBody {
            error: &'static str,
        }
        (self.0, Json(ErrorBody { error: self.1 })).into_response()
    }
}

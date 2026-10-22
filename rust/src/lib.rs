pub mod config;

use std::{sync::Arc, time::Duration};

use axum::{
    Json, Router,
    extract::{DefaultBodyLimit, Path, Query, Request, State, rejection::JsonRejection},
    http::StatusCode,
    middleware::{self, Next},
    response::{IntoResponse, Response},
    routing::get,
};
use deadpool_postgres::Pool;
use serde::{Deserialize, Serialize};
use tokio::sync::Semaphore;
use tokio_postgres::{Row, error::SqlState};

// Limit fruits before joining: applying LIMIT to the joined rows truncates prices.
const LIST: &str = "
    SELECT f.id, f.name, f.description, s.id, s.name, s.currency,
           s.address, s.city, s.country, p.price::DOUBLE PRECISION
    FROM (SELECT id, name, description FROM fruits ORDER BY id LIMIT $1 OFFSET $2) f
    LEFT JOIN store_fruit_prices p ON p.fruit_id = f.id
    LEFT JOIN stores s ON s.id = p.store_id
    ORDER BY f.id, s.id";
const FIND: &str = "
    SELECT f.id, f.name, f.description, s.id, s.name, s.currency,
           s.address, s.city, s.country, p.price::DOUBLE PRECISION
    FROM fruits f
    LEFT JOIN store_fruit_prices p ON p.fruit_id = f.id
    LEFT JOIN stores s ON s.id = p.store_id
    WHERE f.name = $1
    ORDER BY f.id, s.id";
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

#[derive(Debug, Serialize)]
pub struct Store {
    pub id: i64,
    pub name: String,
    pub currency: String,
    pub address: Address,
}

#[derive(Debug, Serialize)]
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
        .with_state(pool)
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
    State(pool): State<Pool>,
    Query(page): Query<Pagination>,
) -> Result<Json<Vec<Fruit>>, ApiError> {
    if !(1..=100).contains(&page.limit) || page.offset < 0 {
        return Err(ApiError::bad_request(
            "limit must be 1 to 100 and offset must be nonnegative",
        ));
    }
    let client = pool.get().await?;
    let statement = client.prepare_cached(LIST).await?;
    let rows = client
        .query(&statement, &[&page.limit, &page.offset])
        .await?;
    Ok(Json(fruits_from_rows(rows)?))
}

async fn find_fruit(
    State(pool): State<Pool>,
    Path(name): Path<String>,
) -> Result<Json<Fruit>, ApiError> {
    let client = pool.get().await?;
    let statement = client.prepare_cached(FIND).await?;
    let rows = client.query(&statement, &[&name]).await?;
    fruits_from_rows(rows)?
        .pop()
        .map(Json)
        .ok_or(ApiError(StatusCode::NOT_FOUND, ""))
}

async fn create_fruit(
    State(pool): State<Pool>,
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
    let client = pool.get().await?;
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

fn fruits_from_rows(rows: Vec<Row>) -> Result<Vec<Fruit>, tokio_postgres::Error> {
    let mut fruits: Vec<Fruit> = Vec::new();
    for row in rows {
        let id: i64 = row.try_get(0)?;
        if fruits.last().is_none_or(|fruit| fruit.id != id) {
            fruits.push(Fruit {
                id,
                name: row.try_get(1)?,
                description: row
                    .try_get::<_, Option<String>>(2)?
                    .filter(|value| !value.is_empty()),
                store_prices: Vec::new(),
            });
        }
        if let Some(store_id) = row.try_get::<_, Option<i64>>(3)? {
            // A price row always belongs to the fruit just added or the previous row's fruit.
            let fruit = fruits
                .last_mut()
                .expect("fruit was added before its prices");
            fruit.store_prices.push(StorePrice {
                store: Store {
                    id: store_id,
                    name: row.try_get(4)?,
                    currency: row.try_get(5)?,
                    address: Address {
                        address: row.try_get(6)?,
                        city: row.try_get(7)?,
                        country: row.try_get(8)?,
                    },
                },
                price: row.try_get(9)?,
            });
        }
    }
    Ok(fruits)
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

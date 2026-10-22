use axum::{
    Router,
    body::{Body, to_bytes},
    http::{Request, StatusCode},
};
use fruits_rust::{config::Config, router};
use serde_json::{Value, json};
use tower::ServiceExt;

fn unconnected_app(max_in_flight: usize) -> Router {
    let config = Config {
        host: "127.0.0.1".into(),
        port: 8080,
        database: "host=localhost dbname=fruits user=fruits".parse().unwrap(),
        pool_size: 1,
        max_in_flight,
    };
    router(config.pool().unwrap(), max_in_flight)
}

async fn request(app: Router, method: &str, path: &str, body: &str) -> (StatusCode, Vec<u8>) {
    let response = app
        .oneshot(
            Request::builder()
                .method(method)
                .uri(path)
                .header("content-type", "application/json")
                .body(Body::from(body.to_owned()))
                .unwrap(),
        )
        .await
        .unwrap();
    let status = response.status();
    let body = to_bytes(response.into_body(), 1024 * 1024).await.unwrap();
    (status, body.to_vec())
}

#[tokio::test]
async fn invalid_payloads_are_rejected_before_acquiring_a_database_connection() {
    let app = unconnected_app(10);
    for payload in [
        "{}".to_owned(),
        "null".to_owned(),
        r#"{"name":null}"#.to_owned(),
        r#"{"name":"  "}"#.to_owned(),
        r#"{"name":"Apple"} {"name":"Pear"}"#.to_owned(),
        json!({"name": "a".repeat(256)}).to_string(),
        json!({"name": "Apple", "description": "a".repeat(256)}).to_string(),
    ] {
        let (status, _) = request(app.clone(), "POST", "/fruits", &payload).await;
        assert_eq!(status, StatusCode::BAD_REQUEST, "payload: {payload}");
    }
}

#[tokio::test]
async fn invalid_pagination_is_rejected_before_acquiring_a_database_connection() {
    let app = unconnected_app(10);
    for query in [
        "limit=0",
        "limit=101",
        "limit=-1",
        "offset=-1",
        "limit=no",
        "offset=9223372036854775808",
    ] {
        let (status, _) = request(app.clone(), "GET", &format!("/fruits?{query}"), "").await;
        assert_eq!(status, StatusCode::BAD_REQUEST, "query: {query}");
    }
}

#[tokio::test]
async fn request_body_and_concurrent_work_are_bounded() {
    let payload = json!({"name": "a".repeat(65 * 1024)}).to_string();
    let (status, _) = request(unconnected_app(10), "POST", "/fruits", &payload).await;
    assert_eq!(status, StatusCode::PAYLOAD_TOO_LARGE);

    let (status, _) = request(unconnected_app(0), "GET", "/fruits", "").await;
    assert_eq!(status, StatusCode::SERVICE_UNAVAILABLE);
}

#[test]
fn response_omits_absent_description_and_empty_prices() {
    let fruit = fruits_rust::Fruit {
        id: 11,
        name: "Grapefruit".into(),
        description: None,
        store_prices: Vec::new(),
    };
    assert_eq!(
        serde_json::to_value(fruit).unwrap(),
        json!({"id": 11, "name": "Grapefruit"})
    );
}

#[tokio::test]
#[ignore = "requires TEST_DATABASE_URL pointing to PostgreSQL with CREATE SCHEMA permission"]
async fn postgres_http_contract() {
    let database: tokio_postgres::Config = std::env::var("TEST_DATABASE_URL")
        .expect("set TEST_DATABASE_URL to a disposable PostgreSQL database")
        .parse()
        .unwrap();
    let (admin, connection) = database.connect(tokio_postgres::NoTls).await.unwrap();
    let connection_task = tokio::spawn(async move { connection.await.unwrap() });
    let unique = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    let schema = format!("rust_test_{}_{}", std::process::id(), unique);
    admin
        .batch_execute(&format!("CREATE SCHEMA {schema}"))
        .await
        .unwrap();

    let mut scoped_database = database;
    scoped_database.options(format!("-c search_path={schema}"));
    let config = Config {
        host: "127.0.0.1".into(),
        port: 8080,
        database: scoped_database,
        pool_size: 2,
        max_in_flight: 10,
    };
    let pool = config.pool().unwrap();
    pool.get()
        .await
        .unwrap()
        .batch_execute(include_str!("../../run/db.sql"))
        .await
        .unwrap();
    let app = router(pool.clone(), 10);

    let (status, body) = request(app.clone(), "GET", "/fruits", "").await;
    assert_eq!(status, StatusCode::OK);
    let default_page: Value = serde_json::from_slice(&body).unwrap();
    assert_eq!(default_page.as_array().unwrap().len(), 50);

    let (status, body) = request(app.clone(), "GET", "/fruits?limit=1&offset=0", "").await;
    assert_eq!(status, StatusCode::OK);
    let first: Value = serde_json::from_slice(&body).unwrap();
    assert_eq!(first.as_array().unwrap().len(), 1);
    assert_eq!(first[0]["name"], "Apple");
    let expected_prices: i64 = pool
        .get()
        .await
        .unwrap()
        .query_one(
            "SELECT COUNT(*) FROM store_fruit_prices WHERE fruit_id = 1",
            &[],
        )
        .await
        .unwrap()
        .get(0);
    assert_eq!(
        first[0]["storePrices"].as_array().unwrap().len(),
        expected_prices as usize
    );
    let prices = first[0]["storePrices"].as_array().unwrap();
    assert!(prices.windows(2).all(|pair| {
        pair[0]["store"]["id"].as_i64().unwrap() < pair[1]["store"]["id"].as_i64().unwrap()
    }));
    assert!(first[0]["storePrices"][0]["store"]["address"]["city"].is_string());
    assert!(first[0]["storePrices"][0]["price"].is_number());

    let (status, body) = request(app.clone(), "GET", "/fruits?limit=1&offset=1", "").await;
    assert_eq!(status, StatusCode::OK);
    let second: Value = serde_json::from_slice(&body).unwrap();
    assert_eq!(second[0]["name"], "Pear");
    let (status, body) =
        request(app.clone(), "GET", "/fruits?offset=9223372036854775807", "").await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(serde_json::from_slice::<Value>(&body).unwrap(), json!([]));

    let (status, body) = request(app.clone(), "GET", "/fruits/Apple", "").await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(serde_json::from_slice::<Value>(&body).unwrap(), first[0]);
    let (status, body) = request(app.clone(), "GET", "/fruits/does-not-exist", "").await;
    assert_eq!(status, StatusCode::NOT_FOUND);
    assert!(body.is_empty());
    let (status, _) = request(app.clone(), "GET", "/fruits/Apple%27%20OR%201%3D1--", "").await;
    assert_eq!(status, StatusCode::NOT_FOUND);

    let (status, body) = request(
        app.clone(),
        "POST",
        "/fruits",
        r#"{"name":"  New Fruit  ","description":"  fresh  ","id":1,"storePrices":[]}"#,
    )
    .await;
    assert_eq!(status, StatusCode::OK);
    let created: Value = serde_json::from_slice(&body).unwrap();
    assert_ne!(created["id"], 1);
    assert_eq!(created["name"], "New Fruit");
    assert_eq!(created["description"], "fresh");
    assert!(created.get("storePrices").is_none());
    let (status, body) = request(app.clone(), "GET", "/fruits/New%20Fruit", "").await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(serde_json::from_slice::<Value>(&body).unwrap(), created);
    let (status, _) = request(app.clone(), "POST", "/fruits", r#"{"name":"New Fruit"}"#).await;
    assert_eq!(status, StatusCode::CONFLICT);

    // A failed transaction must release its connection and leave the pool usable.
    let (status, body) = request(
        app.clone(),
        "POST",
        "/fruits",
        r#"{"name":"Second New Fruit"}"#,
    )
    .await;
    assert_eq!(status, StatusCode::OK);
    assert!(
        serde_json::from_slice::<Value>(&body)
            .unwrap()
            .get("description")
            .is_none()
    );

    drop(app);
    pool.close();
    admin
        .batch_execute(&format!("DROP SCHEMA {schema} CASCADE"))
        .await
        .unwrap();
    drop(admin);
    connection_task.await.unwrap();
}

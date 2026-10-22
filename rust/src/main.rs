use axum::serve::ListenerExt;
use fruits_rust::{config::Config, router};
use tokio::net::TcpListener;
use tracing_subscriber::EnvFilter;

#[tokio::main]
async fn main() -> Result<(), fruits_rust::config::BoxError> {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::try_from_default_env().unwrap_or_else(|_| "info".into()))
        .with_target(false)
        .init();

    let config = Config::from_env()?;
    let pool = config.pool()?;
    // Pool construction is lazy. Fail startup if PostgreSQL cannot be reached.
    pool.get().await?.simple_query("SELECT 1").await?;
    let listener = TcpListener::bind((config.host.as_str(), config.port)).await?;
    tracing::info!(address = %listener.local_addr()?, pool_size = config.pool_size, "server started");
    let listener = listener.tap_io(|stream| {
        if let Err(error) = stream.set_nodelay(true) {
            tracing::warn!(%error, "failed to disable TCP packet coalescing");
        }
    });
    axum::serve(listener, router(pool.clone(), config.max_in_flight))
        .with_graceful_shutdown(shutdown_signal())
        .await?;
    pool.close();
    Ok(())
}

async fn shutdown_signal() {
    let interrupt = async {
        tokio::signal::ctrl_c()
            .await
            .expect("failed to install Ctrl+C handler");
    };
    #[cfg(unix)]
    let terminate = async {
        tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
            .expect("failed to install SIGTERM handler")
            .recv()
            .await;
    };
    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        () = interrupt => {},
        () = terminate => {},
    }
    tracing::info!("draining in-flight requests");
}

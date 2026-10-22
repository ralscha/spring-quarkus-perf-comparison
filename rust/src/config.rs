use std::{env, error::Error, time::Duration};

use deadpool_postgres::{Manager, ManagerConfig, Pool, RecyclingMethod, Runtime};
use tokio_postgres::{Config as PgConfig, NoTls};

pub type BoxError = Box<dyn Error + Send + Sync>;

pub struct Config {
    pub host: String,
    pub port: u16,
    pub database: PgConfig,
    pub pool_size: usize,
    pub max_in_flight: usize,
}

impl Config {
    pub fn from_env() -> Result<Self, BoxError> {
        let database = match env::var("DATABASE_URL") {
            Ok(url) => url.parse::<PgConfig>()?,
            Err(env::VarError::NotPresent) => {
                let mut database = PgConfig::new();
                database
                    .host(value("APP_DATABASE_HOST", "localhost"))
                    .port(number("APP_DATABASE_PORT", 5432)?)
                    .dbname(value("APP_DATABASE_NAME", "fruits"))
                    .user(value("APP_DATABASE_USER", "fruits"))
                    .password(value("APP_DATABASE_PASSWORD", "fruits"));
                database
            }
            Err(error) => return Err(error.into()),
        };
        Ok(Self {
            host: value("APP_SERVER_HOST", "0.0.0.0"),
            port: number("APP_SERVER_PORT", 8080)?,
            database,
            pool_size: number("APP_DATABASE_MAXCONNS", 20)?,
            max_in_flight: number("APP_SERVER_MAXINFLIGHT", 1024)?,
        })
    }

    pub fn pool(&self) -> Result<Pool, BoxError> {
        let mut database = self.database.clone();
        let options = format!(
            "{} -c statement_timeout=5000 -c lock_timeout=3000",
            database.get_options().unwrap_or_default()
        );
        database
            .application_name("fruits-rust")
            .connect_timeout(Duration::from_secs(3))
            .options(&options);
        let manager = Manager::from_config(
            database,
            NoTls,
            ManagerConfig {
                recycling_method: RecyclingMethod::Fast,
            },
        );
        Ok(Pool::builder(manager)
            .runtime(Runtime::Tokio1)
            .max_size(self.pool_size)
            .wait_timeout(Some(Duration::from_secs(3)))
            .create_timeout(Some(Duration::from_secs(3)))
            .recycle_timeout(Some(Duration::from_secs(3)))
            .build()?)
    }
}

fn value(name: &str, default: &str) -> String {
    env::var(name).unwrap_or_else(|_| default.to_owned())
}

fn number<T>(name: &str, default: T) -> Result<T, BoxError>
where
    T: std::str::FromStr + PartialEq + Default,
    T::Err: Error + Send + Sync + 'static,
{
    let number = match env::var(name) {
        Ok(value) => value.parse::<T>()?,
        Err(env::VarError::NotPresent) => default,
        Err(error) => return Err(error.into()),
    };
    if number == T::default() {
        return Err(format!("{name} must be greater than zero").into());
    }
    Ok(number)
}

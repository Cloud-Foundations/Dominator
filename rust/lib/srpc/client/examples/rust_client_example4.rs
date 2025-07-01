/** This example demonstrates how to use the srpc_client Rust bindings.
    RUST_LOG=trace \
    EXAMPLE_4_SRPC_SERVER_HOST=<host> \
    EXAMPLE_4_SRPC_SERVER_PORT=<port> \
    EXAMPLE_4_SRPC_SERVER_ENPOINT=<srpc endpoint> \
    EXAMPLE_4_SRPC_SERVER_CERT=<path to .cert> \
    EXAMPLE_4_SRPC_SERVER_KEY=<path to .key> \
    cargo run --example rust_client_example4
**/
use std::{error::Error, sync::Arc};

use srpc_client::{ClientConfig, CustomError};
use tokio::sync::Mutex;
use tracing::{info, level_filters::LevelFilter};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::builder()
                .with_default_directive(LevelFilter::INFO.into())
                .from_env_lossy(),
        )
        .with(tracing_subscriber::fmt::Layer::default().compact())
        .init();

    info!("Starting client...");

    // Create a new ClientConfig instance
    let config = ClientConfig::new(
        &std::env::var("EXAMPLE_4_SRPC_SERVER_HOST")?,
        std::env::var("EXAMPLE_4_SRPC_SERVER_PORT")?.parse()?,
        &std::env::var("EXAMPLE_4_SRPC_SERVER_ENPOINT")?,
        &std::env::var("EXAMPLE_4_SRPC_SERVER_CERT")?,
        &std::env::var("EXAMPLE_4_SRPC_SERVER_KEY")?,
    );

    // Connect to the server
    let client = config.connect().await?;
    info!("Connected to server");

    let message = "Hypervisor.GetUpdates\n";

    let safe_client = Arc::new(Mutex::new(client));
    let guard = safe_client.lock_owned().await;
    info!("Calling server with message: {:?}", message);
    let mut conn = srpc_client::call(guard, message)
        .await
        .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error>)?;
    let val = conn
        .decode()
        .await
        .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error>)?;
    info!("Received response: {:?}", val);

    let guard = conn.close();

    info!("Calling server with message again: {:?}", message);
    let mut conn2 = srpc_client::call(guard, message)
        .await
        .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error>)?;
    let val = conn2
        .decode()
        .await
        .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error>)?;
    info!("Received response: {:?}", val);
    let _guard = conn2.close();

    Ok(())
}

/** This example demonstrates how to use the srpc_client Rust bindings.
    RUST_LOG=trace \
    EXAMPLE_2_SRPC_SERVER_HOST=<host> \
    EXAMPLE_2_SRPC_SERVER_PORT=<port> \
    EXAMPLE_2_SRPC_SERVER_ENPOINT=<srpc endpoint> \
    EXAMPLE_2_SRPC_SERVER_CERT=<path to .cert> \
    EXAMPLE_2_SRPC_SERVER_KEY=<path to .key> \
    cargo run --example rust_client_example2
**/
use srpc_client::{ClientConfig, ReceiveOptions};
use tracing::{error, info, level_filters::LevelFilter};
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
        &std::env::var("EXAMPLE_2_SRPC_SERVER_HOST")?,
        std::env::var("EXAMPLE_2_SRPC_SERVER_PORT")?.parse()?,
        &std::env::var("EXAMPLE_2_SRPC_SERVER_ENPOINT")?,
        &std::env::var("EXAMPLE_2_SRPC_SERVER_CERT")?,
        &std::env::var("EXAMPLE_2_SRPC_SERVER_KEY")?,
    );

    // Connect to the server
    let client = config.connect().await?;
    info!("Connected to server");

    // Send a message
    let message = "Hypervisor.GetUpdates\n";
    info!("Sending message: {:?}", message);
    client.send_message(message).await?;
    info!("Sent message: {:?}", message);

    // Receive an empty response
    info!("Waiting for empty string response...");
    let mut rx = client
        .receive_message(true, |_| false, &ReceiveOptions::default())
        .await?;
    while let Some(result) = rx.recv().await {
        match result {
            Ok(response) => info!("Received response: {:?}", response),
            Err(e) => error!("Error receiving message: {:?}", e),
        }
    }

    // Receive responses
    let mut rx = client
        .receive_json(|_| false, &ReceiveOptions::default())
        .await?;
    while let Some(result) = rx.recv().await {
        match result {
            Ok(response) => info!("Received response: {:?}", response),
            Err(e) => error!("Error receiving message: {:?}", e),
        }
    }

    Ok(())
}

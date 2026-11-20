/** This example demonstrates how to use the srpc_client Rust bindings.
    RUST_LOG=trace \
    EXAMPLE_1_SRPC_SERVER_HOST=<host> \
    EXAMPLE_1_SRPC_SERVER_PORT=<port> \
    EXAMPLE_1_SRPC_SERVER_ENPOINT=<srpc endpoint> \
    EXAMPLE_1_SRPC_SERVER_CERT=<path to .cert> \
    EXAMPLE_1_SRPC_SERVER_KEY=<path to .key> \
    cargo run --example rust_client_example
**/
use serde_json::json;
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

    // Create a new Client instance
    let client = ClientConfig::new(
        &std::env::var("EXAMPLE_1_SRPC_SERVER_HOST")?,
        std::env::var("EXAMPLE_1_SRPC_SERVER_PORT")?.parse()?,
        &std::env::var("EXAMPLE_1_SRPC_SERVER_ENPOINT")?,
        &std::env::var("EXAMPLE_1_SRPC_SERVER_CERT")?,
        &std::env::var("EXAMPLE_1_SRPC_SERVER_KEY")?,
    );

    // Connect to the server
    let client = client.connect().await?;
    info!("Connected to server");

    // Send a message
    let message = "Hypervisor.ProbeVmPort\n";
    info!("Sending message: {:?}", message);
    client.send_message(message).await?;

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

    // Prepare and send JSON payload
    let json_payload = json!({
        "IpAddress": "<IP Address of VM>",
        "PortNumber": 22
    });

    info!("Sending JSON payload: {:?}", json_payload);
    client.send_json(&json_payload).await?;

    // Receive and parse JSON response
    info!("Waiting for JSON response...");
    let mut rx = client
        .receive_json(|_| false, &ReceiveOptions::default())
        .await?;
    while let Some(result) = rx.recv().await {
        match result {
            Ok(json_response) => info!("Received JSON response: {:?}", json_response),
            Err(e) => error!("Error receiving JSON: {:?}", e),
        }
    }

    Ok(())
}

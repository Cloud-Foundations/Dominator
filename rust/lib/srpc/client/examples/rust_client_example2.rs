use srpc_client::ClientConfig;
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
        "<Hostname or IP of hypervisor>",
        6976,
        "/_SRPC_/TLS/JSON",
        "<Path to Keymaster Certificate file>",
        "<Path to Keymaster Key file>",
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
    let mut rx = client.receive_message(true, |_| false).await?;
    while let Some(result) = rx.recv().await {
        match result {
            Ok(response) => info!("Received response: {:?}", response),
            Err(e) => error!("Error receiving message: {:?}", e),
        }
    }

    // Receive responses
    let mut rx = client.receive_json(|_| false).await?;
    while let Some(result) = rx.recv().await {
        match result {
            Ok(response) => info!("Received response: {:?}", response),
            Err(e) => error!("Error receiving message: {:?}", e),
        }
    }

    Ok(())
}

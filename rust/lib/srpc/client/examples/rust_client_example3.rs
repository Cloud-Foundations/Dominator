/** This example demonstrates how to use the srpc_client Rust bindings.
    RUST_LOG=trace \
    EXAMPLE_3_SRPC_SERVER_HOST=<host> \
    EXAMPLE_3_SRPC_SERVER_PORT=<port> \
    EXAMPLE_3_SRPC_SERVER_ENPOINT=<srpc endpoint> \
    EXAMPLE_3_SRPC_SERVER_CERT=<path to .cert> \
    EXAMPLE_3_SRPC_SERVER_KEY=<path to .key> \
    cargo run --example rust_client_example3
**/
use std::{collections::HashMap, error::Error};

use srpc_client::{ClientConfig, CustomError, ReceiveOptions, SimpleValue};
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
        &std::env::var("EXAMPLE_3_SRPC_SERVER_HOST")?,
        std::env::var("EXAMPLE_3_SRPC_SERVER_PORT")?.parse()?,
        &std::env::var("EXAMPLE_3_SRPC_SERVER_ENPOINT")?,
        &std::env::var("EXAMPLE_3_SRPC_SERVER_CERT")?,
        &std::env::var("EXAMPLE_3_SRPC_SERVER_KEY")?,
    );

    // Connect to the server
    let client = config.connect().await?;
    info!("Connected to server");

    let message = "Hypervisor.ListVMs\n";

    // Send a message
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

    #[derive(Debug, serde::Serialize)]
    struct ListVMsRequest {
        ignore_state_mask: u32,
        owner_groups: Vec<String>,
        owner_users: Vec<String>,
        sort: bool,
        vm_tags_to_match: HashMap<String, String>,
    }

    #[derive(Debug, serde::Deserialize)]
    struct ListVMsResponse {
        ip_addresses: Vec<String>,
    }

    let request = ListVMsRequest {
        ignore_state_mask: 0,
        owner_groups: vec![],
        owner_users: vec![],
        sort: false,
        vm_tags_to_match: HashMap::new(),
    };

    // Send a JSON message
    info!("Sending payload: {:?}", request);
    client.send_json(&serde_json::to_value(&request)?).await?;
    info!("Sent payload: {:?}", request);

    // Receive an empty response
    info!("Waiting for empty string response for payload...");
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
        match result
            .and_then(|response| {
                serde_json::from_value::<ListVMsResponse>(response)
                    .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error + Send>)
            })
            .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error>)
        {
            Ok(response) => info!("Received response: {:?}", response),
            Err(e) => error!("Error receiving message: {:?}", e),
        }
    }

    info!("Sending request_reply: {}", message);
    let res = client
        .request_reply::<SimpleValue>(message, serde_json::to_value(&request)?)
        .await
        .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error>)?;
    info!(
        "Sent request_reply: {}, got reply: {:?}",
        message,
        serde_json::to_string(&res)?
    );

    Ok(())
}

use srpc_client::ClientConfig;
use tokio;
use serde_json::json;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Create a new Client instance
    let client = ClientConfig::new(
        "<Hostname or IP of hypervisor>",
        6976,
        "/_SRPC_/TLS/JSON",
        "<Path to Keymaster Certificate file>",
        "<Path to Keymaster Key file>"
    );

    // Connect to the server
    let client = client.connect().await?;
    println!("Connected to server");

    // Send a message
    let message = "Hypervisor.ProbeVmPort\n";
    println!("Sending message: {:?}", message);
    client.send_message(message).await?;

    // Receive an empty response
    println!("Waiting for empty string response...");
    let mut rx = client.receive_message(true, |_| false).await?;
    while let Some(result) = rx.recv().await {
        match result {
            Ok(response) => println!("Received response: {:?}", response),
            Err(e) => eprintln!("Error receiving message: {:?}", e),
        }
    }

    // Prepare and send JSON payload
    let json_payload = json!({
        "IpAddress": "<IP Address of VM>",
        "PortNumber": 22
    });

    println!("Sending JSON payload: {:?}", json_payload);
    client.send_json(&json_payload).await?;

    // Receive and parse JSON response
    println!("Waiting for JSON response...");
    let mut rx = client.receive_json(|_| false).await?;
    while let Some(result) = rx.recv().await {
        match result {
            Ok(json_response) => println!("Received JSON response: {:?}", json_response),
            Err(e) => eprintln!("Error receiving JSON: {:?}", e),
        }
    }

    Ok(())
}

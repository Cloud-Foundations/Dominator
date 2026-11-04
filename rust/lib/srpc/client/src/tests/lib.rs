use std::{error::Error, num::NonZeroU8};

use crate::{ClientConfig, ConnectedClient, ReceiveOptions};

use rstest::rstest;
use tokio::{
    io::{duplex, AsyncReadExt, AsyncWriteExt, DuplexStream},
    sync::mpsc,
};

fn setup_test_client() -> (ConnectedClient<DuplexStream>, DuplexStream) {
    let (client_stream, server_stream) = duplex(1024);

    let config = ClientConfig::new("example.com", 443, "/", "", "");
    (ConnectedClient::new(config, client_stream), server_stream)
}

fn n_message(num: NonZeroU8) -> impl FnMut(&str) -> bool {
    let mut seen = 0;
    move |_msg: &str| {
        if seen + 1 == num.get() {
            false
        } else {
            seen += 1;
            true
        }
    }
}

fn one_message() -> impl FnMut(&str) -> bool {
    n_message(NonZeroU8::new(1).unwrap())
}

async fn check_message(
    server_message: &str,
    rx: &mut mpsc::Receiver<Result<String, Box<dyn Error + Send>>>,
) {
    if let Some(Ok(received_msg)) = rx.recv().await {
        assert_eq!(received_msg, server_message.trim());
    } else {
        panic!("Did not receive expected message from server");
    }
}

async fn check_server(
    client_message: &str,
    server_stream: &mut DuplexStream,
) -> Result<(), Box<dyn Error>> {
    let mut server_buf = vec![0u8; client_message.len()];
    server_stream.read_exact(&mut server_buf).await?;
    assert_eq!(&server_buf, client_message.as_bytes());
    Ok(())
}

#[test_log::test(rstest)]
#[tokio::test(start_paused = true)]
async fn test_connected_client_send_and_receive() -> Result<(), Box<dyn Error>> {
    let (connected_client, mut server_stream) = setup_test_client();

    let client_message = "Hello from client\n";
    connected_client.send_message(client_message).await?;

    check_server(client_message, &mut server_stream).await?;

    let server_message = "Hello from server\n";
    server_stream.write_all(server_message.as_bytes()).await?;

    let should_continue = one_message();

    let opts = ReceiveOptions::default();
    let mut rx = connected_client
        .receive_message(false, should_continue, &opts)
        .await?;

    check_message(server_message, &mut rx).await;

    Ok(())
}

#[test_log::test(rstest)]
#[tokio::test(start_paused = true)]
async fn test_connected_client_send_and_receive_stream() -> Result<(), Box<dyn Error>> {
    let (connected_client, mut server_stream) = setup_test_client();

    let client_message = "Hello from client\n";
    connected_client.send_message(client_message).await?;

    check_server(client_message, &mut server_stream).await?;

    server_stream.write_all("\n".as_bytes()).await?;

    let should_continue = one_message();

    let opts = ReceiveOptions::default();
    let mut rx = connected_client
        .receive_message(true, should_continue, &opts)
        .await?;

    check_message("", &mut rx).await;

    server_stream.write_all("first\n".as_bytes()).await?;

    server_stream.write_all("second\n".as_bytes()).await?;

    let should_continue = n_message(NonZeroU8::new(2).unwrap());
    let mut rx = connected_client
        .receive_message(false, should_continue, &opts)
        .await?;

    check_message("first", &mut rx).await;
    check_message("second", &mut rx).await;
    Ok(())
}

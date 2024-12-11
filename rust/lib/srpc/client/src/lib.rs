use openssl::ssl::{Ssl, SslConnector, SslMethod, SslVerifyMode};
use serde_json::Value;
use std::error::Error;
use std::fmt;
use std::pin::Pin;
use std::sync::Arc;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream;
use tokio::sync::{mpsc, Mutex};
use tokio::time::{timeout, Duration};
use tokio_openssl::SslStream;
use tracing::debug;

// Custom error type
#[derive(Debug)]
struct CustomError(String);

impl fmt::Display for CustomError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl Error for CustomError {}

#[derive(Clone)]
pub struct ClientConfig {
    host: String,
    port: u16,
    path: String,
    cert: String,
    key: String,
}

pub struct ConnectedClient {
    pub connection_params: ClientConfig,
    stream: Connected,
}

type Connected = Arc<Mutex<SslStream<TcpStream>>>;
impl ClientConfig {
    pub fn new(host: &str, port: u16, path: &str, cert: &str, key: &str) -> Self {
        ClientConfig {
            host: host.to_string(),
            port,
            path: path.to_string(),
            cert: cert.to_string(),
            key: key.to_string(),
        }
    }

    pub async fn connect(self) -> Result<ConnectedClient, Box<dyn Error>> {
        debug!("Attempting to connect to {}:{}...", self.host, self.port);

        let connect_timeout = Duration::from_secs(10);
        let tcp_stream = match timeout(
            connect_timeout,
            TcpStream::connect(format!("{}:{}", self.host, self.port)),
        )
        .await
        {
            Ok(Ok(stream)) => stream,
            Ok(Err(e)) => return Err(format!("Failed to connect: {}", e).into()),
            Err(_) => return Err("Connection attempt timed out".into()),
        };
        debug!("TCP connection established");

        debug!("Performing HTTP CONNECT...");
        self.do_http_connect(&tcp_stream).await?;
        debug!("HTTP CONNECT successful");

        debug!("Starting TLS handshake...");
        let mut connector = SslConnector::builder(SslMethod::tls())?;
        connector.set_verify(SslVerifyMode::NONE);

        if !self.cert.is_empty() && !self.key.is_empty() {
            connector.set_certificate_file(&self.cert, openssl::ssl::SslFiletype::PEM)?;
            connector.set_private_key_file(&self.key, openssl::ssl::SslFiletype::PEM)?;
        }

        let ssl = Ssl::new(connector.build().context())?;
        let mut stream = SslStream::new(ssl, tcp_stream)?;

        debug!("Performing TLS handshake...");
        Pin::new(&mut stream).connect().await?;
        debug!("TLS handshake completed");

        debug!("Connection fully established");

        Ok(ConnectedClient {
            connection_params: self,
            stream: Arc::new(Mutex::new(stream)),
        })
    }

    async fn do_http_connect(&self, stream: &TcpStream) -> Result<(), Box<dyn Error>> {
        let connect_request = format!("CONNECT {} HTTP/1.0\r\n\r\n", self.path);
        debug!("Sending HTTP CONNECT request: {:?}", connect_request);
        stream.try_write(connect_request.as_bytes())?;
        debug!("HTTP CONNECT request sent");

        let read_timeout = Duration::from_secs(10);
        let start_time = std::time::Instant::now();
        let mut buffer = Vec::new();

        while start_time.elapsed() < read_timeout {
            match stream.try_read_buf(&mut buffer) {
                Ok(0) => {
                    if !buffer.is_empty() {
                        break;
                    }
                    tokio::time::sleep(Duration::from_millis(10)).await;
                }
                Ok(_) => {
                    if buffer.ends_with(b"\r\n\r\n") {
                        break;
                    }
                }
                Err(ref e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                    tokio::time::sleep(Duration::from_millis(10)).await;
                }
                Err(e) => return Err(format!("Error reading HTTP CONNECT response: {}", e).into()),
            }
        }

        if buffer.is_empty() {
            return Err("Timeout while waiting for HTTP CONNECT response".into());
        }

        let response = String::from_utf8_lossy(&buffer);
        debug!("Received HTTP CONNECT response: {:?}", response);
        if response.starts_with("HTTP/1.0 200") || response.starts_with("HTTP/1.1 200") {
            debug!("HTTP CONNECT completed successfully");
            Ok(())
        } else {
            Err(format!("Unexpected HTTP response: {}", response).into())
        }
    }
}

impl ConnectedClient {
    pub async fn send_message(&self, message: &str) -> Result<(), Box<dyn Error>> {
        let stream = self.stream.lock().await;
        let mut pinned = Pin::new(stream);
        pinned.as_mut().write_all(message.as_bytes()).await?;
        pinned.as_mut().flush().await?;
        Ok(())
    }

    pub async fn receive_message<F>(
        &self,
        expect_empty: bool,
        mut should_continue: F,
    ) -> Result<mpsc::Receiver<Result<String, Box<dyn Error + Send>>>, Box<dyn Error>>
    where
        F: FnMut(&str) -> bool + Send + 'static,
    {
        let stream_clone = self.stream.clone();
        let (tx, rx) = mpsc::channel(100);

        tokio::spawn(async move {
            loop {
                let mut stream = stream_clone.lock().await;
                let mut response = String::new();
                loop {
                    let mut buf = [0; 1024];
                    match stream.read(&mut buf).await {
                        Ok(0) => {
                            let _ = tx.send(Ok(String::new())).await;
                            return;
                        }
                        Ok(n) => {
                            response.push_str(&String::from_utf8_lossy(&buf[..n]));
                            if response.ends_with('\n') {
                                break;
                            }
                        }
                        Err(e) => {
                            let _ = tx.send(Err(Box::new(e) as Box<dyn Error + Send>)).await;
                            return;
                        }
                    }
                }
                let response = response.trim().to_string();

                if expect_empty && !response.is_empty() {
                    let _ = tx
                        .send(Err(Box::new(CustomError(format!(
                            "Expected empty string, got: {:?}",
                            response
                        ))) as Box<dyn Error + Send>))
                        .await;
                    return;
                }

                let _ = tx.send(Ok(response.clone())).await;

                if !should_continue(&response) {
                    break;
                }
            }
        });

        Ok(rx)
    }

    pub async fn send_json(&self, payload: &Value) -> Result<(), Box<dyn Error>> {
        let json_string = payload.to_string() + "\n";
        self.send_message(&json_string).await
    }

    pub async fn receive_json<F>(
        &self,
        should_continue: F,
    ) -> Result<mpsc::Receiver<Result<Value, Box<dyn Error + Send>>>, Box<dyn Error>>
    where
        F: FnMut(&str) -> bool + Send + 'static,
    {
        let mut rx = self.receive_message(false, should_continue).await?;
        let (tx, new_rx) = mpsc::channel(100);

        tokio::spawn(async move {
            while let Some(result) = rx.recv().await {
                match result {
                    Ok(json_str) => match serde_json::from_str(&json_str) {
                        Ok(json_value) => {
                            if let Err(_) = tx.send(Ok(json_value)).await {
                                break;
                            }
                        }
                        Err(e) => {
                            let _ = tx.send(Err(Box::new(e) as Box<dyn Error + Send>)).await;
                        }
                    },
                    Err(e) => {
                        let _ = tx.send(Err(e)).await;
                    }
                }
            }
        });

        Ok(new_rx)
    }
}

#[cfg(feature = "python")]
mod python_bindings;

#[cfg(feature = "python")]
use pyo3::prelude::*;

#[cfg(feature = "python")]
#[pymodule]
fn srpc_client(_py: Python, m: &PyModule) -> PyResult<()> {
    use tracing::level_filters::LevelFilter;
    use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

    tracing_subscriber::registry()
        .with(tracing_subscriber::EnvFilter::builder().with_default_directive(LevelFilter::INFO.into()).from_env_lossy())
        .with(tracing_subscriber::fmt::Layer::default().compact())
        .init();

    m.add_class::<python_bindings::SrpcClientConfig>()?;
    m.add_class::<python_bindings::ConnectedSrpcClient>()?;
    Ok(())
}

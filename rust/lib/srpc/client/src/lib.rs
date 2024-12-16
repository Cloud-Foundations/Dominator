use async_trait::async_trait;
use chunk_limiter::ChunkLimiter;
use futures::StreamExt;
use openssl::ssl::{Ssl, SslConnector, SslMethod, SslVerifyMode};
use serde_json::Value;
use std::error::Error;
use std::fmt;
use std::pin::Pin;
use std::sync::Arc;
use tokio::io::{AsyncRead, AsyncWrite, AsyncWriteExt, BufReader};
use tokio::net::TcpStream;
use tokio::sync::{mpsc, Mutex};
use tokio::time::{timeout, Duration};
use tokio_openssl::SslStream;
use tokio_util::codec::{FramedRead, LinesCodec};
use tracing::debug;

mod chunk_limiter;
#[cfg(test)]
mod tests;

// Custom error type
#[derive(Debug)]
pub struct CustomError(pub String);

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

#[cfg_attr(feature = "python", derive(FromPyObject))]
pub struct ReceiveOptions {
    channel_buffer_size: usize,
    max_chunk_size: usize,
    read_next_line_duration: Duration,
    should_continue_on_timeout: bool,
}

impl ReceiveOptions {
    pub fn new(
        channel_buffer_size: usize,
        max_chunk_size: usize,
        read_next_line_duration: Duration,
        should_continue_on_timeout: bool,
    ) -> Self {
        ReceiveOptions {
            channel_buffer_size,
            max_chunk_size,
            read_next_line_duration,
            should_continue_on_timeout,
        }
    }
}

impl Default for ReceiveOptions {
    fn default() -> Self {
        ReceiveOptions {
            channel_buffer_size: 100,
            max_chunk_size: 16384,
            read_next_line_duration: Duration::from_secs(10),
            should_continue_on_timeout: true,
        }
    }
}

pub struct ConnectedClient<T>
where
    T: AsyncRead + AsyncWrite + Unpin + Send + 'static,
{
    pub connection_params: ClientConfig,
    stream: Arc<Mutex<T>>,
}

#[async_trait]
pub trait RequestReply {
    type Request;
    type Reply;

    async fn request_reply<T>(
        client: &ConnectedClient<T>,
        payload: Self::Request,
    ) -> Result<Self::Reply, Box<dyn Error + Send>>
    where
        T: AsyncRead + AsyncWrite + Unpin + Send + 'static;
}

pub struct SimpleValue;

#[async_trait]
impl RequestReply for SimpleValue {
    type Request = serde_json::Value;
    type Reply = serde_json::Value;

    async fn request_reply<T>(
        client: &ConnectedClient<T>,
        payload: Self::Request,
    ) -> Result<Self::Reply, Box<dyn Error + Send>>
    where
        T: AsyncRead + AsyncWrite + Unpin + Send + 'static,
    {
        client.send_json_and_check(&payload).await?;

        let mut rx = client
            .receive_json(|_| false, &ReceiveOptions::default())
            .await
            .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error + Send>)?;
        let json_value = rx.recv().await.ok_or_else(|| {
            Box::new(CustomError("Expected JSON value".to_string())) as Box<dyn Error + Send>
        })??;
        Ok(json_value)
    }
}

pub struct StreamValue;

#[async_trait]
impl RequestReply for StreamValue {
    type Request = serde_json::Value;
    type Reply = mpsc::Receiver<Result<serde_json::Value, Box<dyn Error + Send>>>;

    async fn request_reply<T>(
        client: &ConnectedClient<T>,
        payload: Self::Request,
    ) -> Result<Self::Reply, Box<dyn Error + Send>>
    where
        T: AsyncRead + AsyncWrite + Unpin + Send + 'static,
    {
        client.send_json_and_check(&payload).await?;

        let rx = client
            .receive_json(|_| true, &ReceiveOptions::default())
            .await
            .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error + Send>)?;
        Ok(rx)
    }
}

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

    pub async fn connect(self) -> Result<ConnectedClient<SslStream<TcpStream>>, Box<dyn Error>> {
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

        Ok(ConnectedClient::new(self, stream))
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

impl<T> ConnectedClient<T>
where
    T: AsyncRead + AsyncWrite + Unpin + Send + 'static,
{
    pub fn new(connection_params: ClientConfig, stream: T) -> Self {
        ConnectedClient {
            connection_params,
            stream: Arc::new(Mutex::new(stream)),
        }
    }

    pub async fn request_reply<R>(
        &self,
        method: &str,
        payload: R::Request,
    ) -> Result<R::Reply, Box<dyn Error + Send>>
    where
        R: RequestReply,
    {
        self.send_message(method)
            .await
            .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error + Send>)?;
        let mut rx = self
            .receive_message(true, |_| false, &ReceiveOptions::default())
            .await
            .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error + Send>)?;
        if rx
            .recv()
            .await
            .ok_or_else(|| {
                Box::new(CustomError("Expected response".to_string())) as Box<dyn Error + Send>
            })??
            .is_empty()
        {
        } else {
            return Err(Box::new(CustomError("Expected empty line".to_string())));
        }

        R::request_reply(self, payload).await
    }

    pub async fn send_message(&self, message: &str) -> Result<(), Box<dyn Error>> {
        let stream = self.stream.lock().await;
        let mut pinned = Pin::new(stream);
        pinned.as_mut().write_all(message.as_bytes()).await?;
        pinned.as_mut().flush().await?;
        Ok(())
    }

    pub async fn send_message_and_check(&self, message: &str) -> Result<(), Box<dyn Error + Send>> {
        self.send_message(message)
            .await
            .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error + Send>)?;
        let mut rx = self
            .receive_message(true, |_| false, &ReceiveOptions::default())
            .await
            .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error + Send>)?;
        if rx
            .recv()
            .await
            .ok_or_else(|| {
                Box::new(CustomError("Expected response".to_string())) as Box<dyn Error + Send>
            })??
            .is_empty()
        {
        } else {
            return Err(Box::new(CustomError("Expected empty line".to_string())));
        }

        Ok(())
    }

    pub async fn receive_message<F>(
        &self,
        expect_empty: bool,
        mut should_continue: F,
        opts: &ReceiveOptions,
    ) -> Result<mpsc::Receiver<Result<String, Box<dyn Error + Send>>>, Box<dyn Error>>
    where
        F: FnMut(&str) -> bool + Send + 'static,
    {
        let stream = Arc::clone(&self.stream);
        let (tx, rx) = mpsc::channel(opts.channel_buffer_size);
        let max_chunk_size = if expect_empty { 1 } else { opts.max_chunk_size };
        let read_next_line_duration = opts.read_next_line_duration;
        let should_continue_on_timeout = opts.should_continue_on_timeout;

        tokio::spawn(async move {
            let mut guard = stream.lock().await;
            let limited_reader = ChunkLimiter::new(&mut *guard, max_chunk_size);
            let buf_reader = BufReader::new(limited_reader);
            let mut framed = FramedRead::new(buf_reader, LinesCodec::new());

            loop {
                let result = timeout(read_next_line_duration, framed.next()).await;
                match result {
                    Ok(Some(line_res)) => {
                        let line_res = line_res.map_err(|e| Box::new(e) as Box<dyn Error + Send>);

                        match line_res {
                            Ok(line) => {
                                if expect_empty && !line.is_empty() {
                                    let _ = tx
                                        .send(Err(Box::new(CustomError(format!(
                                            "Expected empty line, got: {:?}",
                                            line
                                        )))
                                            as Box<dyn Error + Send>))
                                        .await;
                                    break;
                                }

                                let _ = tx.send(Ok(line.clone())).await;

                                if !should_continue(&line) {
                                    break;
                                }
                            }
                            Err(err) => {
                                let _ = tx.send(Err(err)).await;
                                break;
                            }
                        }
                    }
                    Ok(None) => {
                        break;
                    }
                    Err(_) => {
                        if !should_continue_on_timeout {
                            break;
                        }
                    }
                }
            }
        });

        Ok(rx)
    }

    pub async fn send_json(&self, payload: &Value) -> Result<(), Box<dyn Error>> {
        let json_string = payload.to_string() + "\n";
        self.send_message(&json_string).await
    }

    pub async fn send_json_and_check(&self, payload: &Value) -> Result<(), Box<dyn Error + Send>> {
        self.send_json(payload)
            .await
            .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error + Send>)?;
        let mut rx = self
            .receive_message(true, |_| false, &ReceiveOptions::default())
            .await
            .map_err(|e| Box::new(CustomError(e.to_string())) as Box<dyn Error + Send>)?;
        if rx
            .recv()
            .await
            .ok_or_else(|| {
                Box::new(CustomError("Expected response".to_string())) as Box<dyn Error + Send>
            })??
            .is_empty()
        {
        } else {
            return Err(Box::new(CustomError("Expected empty line".to_string())));
        }

        Ok(())
    }

    pub async fn receive_json<F>(
        &self,
        should_continue: F,
        opts: &ReceiveOptions,
    ) -> Result<mpsc::Receiver<Result<Value, Box<dyn Error + Send>>>, Box<dyn Error>>
    where
        F: FnMut(&str) -> bool + Send + 'static,
    {
        let mut rx = self.receive_message(false, should_continue, opts).await?;
        let (tx, new_rx) = mpsc::channel(opts.channel_buffer_size);

        tokio::spawn(async move {
            while let Some(result) = rx.recv().await {
                match result.and_then(|json_str| {
                    serde_json::from_str(&json_str)
                        .map_err(|e| Box::new(e) as Box<dyn Error + Send>)
                }) {
                    Ok(json_value) => {
                        if let Err(_) = tx.send(Ok(json_value)).await {
                            break;
                        }
                    }
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
fn srpc_client(_py: Python, m: &Bound<'_, PyModule>) -> PyResult<()> {
    use tracing::level_filters::LevelFilter;
    use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::builder()
                .with_default_directive(LevelFilter::INFO.into())
                .from_env_lossy(),
        )
        .with(tracing_subscriber::fmt::Layer::default().compact())
        .init();

    m.add_class::<python_bindings::SrpcClientConfig>()?;
    m.add_class::<python_bindings::ConnectedSrpcClient>()?;
    Ok(())
}

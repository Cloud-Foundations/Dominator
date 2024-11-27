use tokio::net::TcpStream;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use std::error::Error;
use std::fmt;
use openssl::ssl::{SslMethod, SslConnector, SslVerifyMode, Ssl};
use serde_json::Value;
use tokio_openssl::SslStream;
use tokio::time::{timeout, Duration};
use std::sync::Arc;
use tokio::sync::{Mutex, mpsc};
use std::pin::Pin;

// Custom error type
#[derive(Debug)]
struct CustomError(String);

impl fmt::Display for CustomError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl Error for CustomError {}

pub struct Client {
    host: String,
    port: u16,
    path: String,
    cert: String,
    key: String,
    stream: Arc<Mutex<Option<SslStream<TcpStream>>>>,
}

impl Client {
    pub fn new(host: &str, port: u16, path: &str, cert: &str, key: &str) -> Self {
        Client {
            host: host.to_string(),
            port,
            path: path.to_string(),
            cert: cert.to_string(),
            key: key.to_string(),
            stream: Arc::new(Mutex::new(None)),
        }
    }

    pub async fn connect(&self) -> Result<(), Box<dyn Error>> {
        // println!("Attempting to connect to {}:{}...", self.host, self.port);
        
        let connect_timeout = Duration::from_secs(10);
        let tcp_stream = match timeout(connect_timeout, 
            TcpStream::connect(format!("{}:{}", self.host, self.port))
        ).await {
            Ok(Ok(stream)) => stream,
            Ok(Err(e)) => return Err(format!("Failed to connect: {}", e).into()),
            Err(_) => return Err("Connection attempt timed out".into()),
        };
        // println!("TCP connection established");
    
        // println!("Performing HTTP CONNECT...");
        self.do_http_connect(&tcp_stream).await?;
        // println!("HTTP CONNECT successful");
    
        // println!("Starting TLS handshake...");
        let mut connector = SslConnector::builder(SslMethod::tls())?;
        connector.set_verify(SslVerifyMode::NONE);

        if !self.cert.is_empty() && !self.key.is_empty() {
            connector.set_certificate_file(&self.cert, openssl::ssl::SslFiletype::PEM)?;
            connector.set_private_key_file(&self.key, openssl::ssl::SslFiletype::PEM)?;
        }
    
        let ssl = Ssl::new(connector.build().context())?;
        let mut stream = SslStream::new(ssl, tcp_stream)?;
    
        // println!("Performing TLS handshake...");
        Pin::new(&mut stream).connect().await?;
        // println!("TLS handshake completed");
    
        let mut lock = self.stream.lock().await;
        *lock = Some(stream);
        // println!("Connection fully established");
    
        Ok(())
    }

    async fn do_http_connect(&self, stream: &TcpStream) -> Result<(), Box<dyn Error>> {
        let connect_request = format!("CONNECT {} HTTP/1.0\r\n\r\n", self.path);
        // println!("Sending HTTP CONNECT request: {:?}", connect_request);
        stream.try_write(connect_request.as_bytes())?;
        // println!("HTTP CONNECT request sent");
    
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
        // println!("Received HTTP CONNECT response: {:?}", response);
        if response.starts_with("HTTP/1.0 200") || response.starts_with("HTTP/1.1 200") {
            // println!("HTTP CONNECT completed successfully");
            Ok(())
        } else {
            Err(format!("Unexpected HTTP response: {}", response).into())
        }
    }

    pub async fn send_message(&self, message: &str) -> Result<(), Box<dyn Error>> {
        let mut lock = self.stream.lock().await;
        if let Some(stream) = lock.as_mut() {
            let mut pinned = Pin::new(stream);
            pinned.as_mut().write_all(message.as_bytes()).await?;
            pinned.as_mut().flush().await?;
            Ok(())
        } else {
            Err("Not connected".into())
        }
    }

    pub async fn receive_message<F>(&self, expect_empty: bool, mut should_continue: F) -> Result<mpsc::Receiver<Result<String, Box<dyn Error + Send>>>, Box<dyn Error>>
    where
        F: FnMut(&str) -> bool + Send + 'static,
    {
        let stream_clone = self.stream.clone();
        let (tx, rx) = mpsc::channel(100);

        tokio::spawn(async move {
            loop {
                let mut lock = stream_clone.lock().await;
                if let Some(stream) = lock.as_mut() {
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
                        let _ = tx.send(Err(Box::new(CustomError(format!("Expected empty string, got: {:?}", response))) as Box<dyn Error + Send>)).await;
                        return;
                    }
                    
                    let _ = tx.send(Ok(response.clone())).await;
                    
                    if !should_continue(&response) {
                        break;
                    }
                } else {
                    let _ = tx.send(Err(Box::new(CustomError("Not connected".to_string())) as Box<dyn Error + Send>)).await;
                    return;
                }
            }
        });

        Ok(rx)
    }

    pub async fn send_json(&self, payload: &Value) -> Result<(), Box<dyn Error>> {
        let json_string = payload.to_string() + "\n";
        self.send_message(&json_string).await
    }

    pub async fn receive_json<F>(&self, should_continue: F) -> Result<mpsc::Receiver<Result<Value, Box<dyn Error + Send>>>, Box<dyn Error>>
    where
        F: FnMut(&str) -> bool + Send + 'static,
    {
        let mut rx = self.receive_message(false, should_continue).await?;
        let (tx, new_rx) = mpsc::channel(100);

        tokio::spawn(async move {
            while let Some(result) = rx.recv().await {
                match result {
                    Ok(json_str) => {
                        match serde_json::from_str(&json_str) {
                            Ok(json_value) => {
                                if let Err(_) = tx.send(Ok(json_value)).await {
                                    break;
                                }
                            }
                            Err(e) => {
                                let _ = tx.send(Err(Box::new(e) as Box<dyn Error + Send>)).await;
                            }
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
fn srpc_client(_py: Python, m: &PyModule) -> PyResult<()> {
    m.add_class::<python_bindings::SrpcClient>()?;
    Ok(())
}

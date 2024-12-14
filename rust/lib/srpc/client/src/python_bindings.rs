use crate::{ClientConfig, ConnectedClient, ReceiveOptions};
use futures::{Stream, StreamExt};
use pyo3::exceptions::{PyRuntimeError, PyStopAsyncIteration};
use pyo3::prelude::*;
use serde_json::Value;
use std::{
    pin::Pin,
    sync::Arc,
    task::{Context, Poll},
};
use tokio::net::TcpStream;
use tokio::sync::{mpsc, Mutex};
use tokio_openssl::SslStream;

#[pyclass]
pub struct SrpcClientConfig(ClientConfig);

#[pyclass]
pub struct ConnectedSrpcClient(Arc<Mutex<ConnectedClient<SslStream<TcpStream>>>>);

#[pymethods]
impl SrpcClientConfig {
    #[new]
    pub fn new(host: &str, port: u16, path: &str, cert: &str, key: &str) -> Self {
        SrpcClientConfig(ClientConfig::new(host, port, path, cert, key))
    }

    pub fn connect<'p>(&self, py: Python<'p>) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        pyo3_asyncio::tokio::future_into_py(py, async {
            client
                .connect()
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))
                .map(|c| ConnectedSrpcClient(Arc::new(Mutex::new(c))))
        })
    }
}

struct Streamer {
    rx: mpsc::Receiver<Result<String, Box<dyn std::error::Error + Send>>>,
}

impl Streamer {
    fn new(rx: mpsc::Receiver<Result<String, Box<dyn std::error::Error + Send>>>) -> Self {
        Streamer { rx }
    }
}

impl Stream for Streamer {
    type Item = Result<String, Box<dyn std::error::Error + Send>>;

    fn poll_next(self: Pin<&mut Self>, cx: &mut Context) -> Poll<Option<Self::Item>> {
        self.get_mut().rx.poll_recv(cx)
    }
}

#[pyo3::pyclass]
struct PyStream {
    pub streamer: Arc<Mutex<Streamer>>,
}

impl PyStream {
    fn new(streamer: Streamer) -> Self {
        PyStream {
            streamer: Arc::new(Mutex::new(streamer)),
        }
    }
}

#[pymethods]
impl PyStream {
    fn __aiter__(slf: PyRef<'_, Self>) -> PyRef<'_, Self> {
        slf
    }

    fn __anext__(&self, py: Python) -> PyResult<Option<PyObject>> {
        let streamer = self.streamer.clone();
        let future = pyo3_asyncio::tokio::future_into_py(py, async move {
            let val = streamer.lock().await.next().await;
            match val {
                Some(Ok(val)) => Ok(val),
                Some(Err(val)) => Err(PyRuntimeError::new_err(val.to_string())),
                None => Err(PyStopAsyncIteration::new_err("The iterator is exhausted")),
            }
        });
        Ok(Some(future?.into()))
    }
}

struct ValueStreamer {
    rx: mpsc::Receiver<Result<serde_json::Value, Box<dyn std::error::Error + Send>>>,
}

impl ValueStreamer {
    fn new(
        rx: mpsc::Receiver<Result<serde_json::Value, Box<dyn std::error::Error + Send>>>,
    ) -> Self {
        ValueStreamer { rx }
    }
}

impl Stream for ValueStreamer {
    type Item = Result<serde_json::Value, Box<dyn std::error::Error + Send>>;

    fn poll_next(self: Pin<&mut Self>, cx: &mut Context) -> Poll<Option<Self::Item>> {
        self.get_mut().rx.poll_recv(cx)
    }
}

#[pyo3::pyclass]
struct PyValueStream {
    pub streamer: Arc<Mutex<ValueStreamer>>,
}

impl PyValueStream {
    fn new(streamer: ValueStreamer) -> Self {
        PyValueStream {
            streamer: Arc::new(Mutex::new(streamer)),
        }
    }
}

#[pymethods]
impl PyValueStream {
    fn __aiter__(slf: PyRef<'_, Self>) -> PyRef<'_, Self> {
        slf
    }

    fn __anext__(&self, py: Python) -> PyResult<Option<PyObject>> {
        let streamer = self.streamer.clone();
        let future = pyo3_asyncio::tokio::future_into_py(py, async move {
            let val = streamer.lock().await.next().await;
            match val {
                Some(Ok(val)) => Ok(val.to_string()),
                Some(Err(val)) => Err(PyRuntimeError::new_err(val.to_string())),
                None => Err(PyStopAsyncIteration::new_err("The iterator is exhausted")),
            }
        });
        Ok(Some(future?.into()))
    }
}

#[pymethods]
impl ConnectedSrpcClient {
    pub fn send_message<'p>(&self, py: Python<'p>, message: String) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        pyo3_asyncio::tokio::future_into_py(py, async move {
            client
                .lock()
                .await
                .send_message(&message)
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))
        })
    }

    pub fn receive_message<'p>(
        &self,
        py: Python<'p>,
        expect_empty: bool,
        should_continue: bool,
    ) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        pyo3_asyncio::tokio::future_into_py(py, async move {
            let rx = client
                .lock()
                .await
                .receive_message(
                    expect_empty,
                    move |_| should_continue,
                    &ReceiveOptions::default(),
                )
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;

            Ok(Python::with_gil(|_py| PyStream::new(Streamer::new(rx))))
        })
    }

    pub fn receive_message_cb<'p>(
        &self,
        py: Python<'p>,
        expect_empty: bool,
        should_continue: &PyAny,
    ) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        let should_continue = should_continue.to_object(py);

        pyo3_asyncio::tokio::future_into_py(py, async move {
            let should_continue = move |response: &str| -> bool {
                Python::with_gil(|py| {
                    let func = should_continue.as_ref(py);

                    func.call1((response,))
                        .and_then(|v| v.extract::<bool>())
                        .unwrap_or(false)
                })
            };
            let rx = client
                .lock()
                .await
                .receive_message(expect_empty, should_continue, &ReceiveOptions::default())
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;

            Ok(Python::with_gil(|_py| PyStream::new(Streamer::new(rx))))
        })
    }

    pub fn send_json<'p>(&self, py: Python<'p>, payload: String) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        pyo3_asyncio::tokio::future_into_py(py, async move {
            let value: Value = serde_json::from_str(&payload)
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;
            client
                .lock()
                .await
                .send_json(&value)
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))
        })
    }

    pub fn receive_json<'p>(&self, py: Python<'p>, should_continue: bool) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        pyo3_asyncio::tokio::future_into_py(py, async move {
            let rx = client
                .lock()
                .await
                .receive_json(move |_| should_continue, &ReceiveOptions::default())
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;

            Ok(Python::with_gil(|_py| {
                PyValueStream::new(ValueStreamer::new(rx))
            }))
        })
    }

    pub fn receive_json_cb<'p>(
        &self,
        py: Python<'p>,
        should_continue: &PyAny,
    ) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        let should_continue = should_continue.to_object(py);

        pyo3_asyncio::tokio::future_into_py(py, async move {
            let should_continue = move |response: &str| -> bool {
                Python::with_gil(|py| {
                    let func = should_continue.as_ref(py);

                    func.call1((response,))
                        .and_then(|v| v.extract::<bool>())
                        .unwrap_or(false)
                })
            };
            let rx = client
                .lock()
                .await
                .receive_json(should_continue, &ReceiveOptions::default())
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;

            Ok(Python::with_gil(|_py| {
                PyValueStream::new(ValueStreamer::new(rx))
            }))
        })
    }
}

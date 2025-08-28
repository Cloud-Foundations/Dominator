use crate::{ClientConfig, Conn, ConnectedClient, ReceiveOptions, SimpleValue};
use futures::{Stream, StreamExt};
use pyo3::exceptions::{PyRuntimeError, PyStopAsyncIteration};
use pyo3::prelude::*;
use pyo3::types::PyFunction;
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

#[pyclass]
pub struct SrpcMethodCallConn(Arc<Mutex<Conn<SslStream<TcpStream>>>>);

#[pymethods]
impl SrpcClientConfig {
    #[new]
    pub fn new(host: &str, port: u16, path: &str, cert: &str, key: &str) -> Self {
        SrpcClientConfig(ClientConfig::new(host, port, path, cert, key))
    }

    pub fn connect<'p>(&self, py: Python<'p>) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();
        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            client
                .connect()
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))
                .map(|c| {
                    ConnectedSrpcClient(Arc::new(Mutex::new(c)))
                })
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
        let future = pyo3_async_runtimes::tokio::future_into_py(py, async move {
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
        let future = pyo3_async_runtimes::tokio::future_into_py(py, async move {
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
    pub fn send_message<'p>(
        &'p self,
        py: Python<'p>,
        message: String,
    ) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();
        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let client = client.lock().await;
            client
                .send_message(&message)
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))
        })
    }

    pub fn send_message_and_check<'p>(
        &'p self,
        py: Python<'p>,
        message: String,
    ) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();
        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let client = client.lock().await;
            client
                .send_message_and_check(&message)
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))
        })
    }

    pub fn receive_message<'p>(
        &self,
        py: Python<'p>,
        expect_empty: bool,
        should_continue: bool,
    ) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();
        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let client = client.lock().await;
            let rx = client
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
        should_continue: Py<PyFunction>,
    ) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();
        let should_continue = should_continue.clone_ref(py);

        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let should_continue = move |response: &str| -> bool {
                Python::with_gil(|py| {
                    should_continue
                        .call1(py, (response,))
                        .and_then(|v| v.extract::<bool>(py))
                        .unwrap_or(false)
                })
            };
            let client = client.lock().await;
            let rx = client
                .receive_message(expect_empty, should_continue, &ReceiveOptions::default())
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;

            Ok(Python::with_gil(|_py| PyStream::new(Streamer::new(rx))))
        })
    }

    pub fn send_json<'p>(&self, py: Python<'p>, payload: String) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();
        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let value: Value = serde_json::from_str(&payload)
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;
            let client = client.lock().await;
            client
                .send_json(&value)
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))
        })
    }

    pub fn send_json_and_check<'p>(
        &self,
        py: Python<'p>,
        payload: String,
    ) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();
        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let value: Value = serde_json::from_str(&payload)
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;
            let client = client.lock().await;
            client
                .send_json_and_check(&value)
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))
        })
    }

    pub fn receive_json<'p>(
        &self,
        py: Python<'p>,
        should_continue: bool,
    ) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();
        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let client = client.lock().await;
            let rx = client
                .receive_json(move |_| should_continue, &ReceiveOptions::default())
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;

            Ok(Python::with_gil(|_py| {
                PyValueStream::new(ValueStreamer::new(rx))
            }))
        })
    }

    #[pyo3(signature = (should_continue, opts=None))]
    pub fn receive_json_cb<'p>(
        &self,
        py: Python<'p>,
        should_continue: Py<PyFunction>,
        opts: Option<ReceiveOptions>,
    ) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();
        let should_continue = should_continue.clone_ref(py);

        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let should_continue = move |response: &str| -> bool {
                Python::with_gil(|py| {
                    should_continue
                        .call1(py, (response,))
                        .and_then(|v| v.extract::<bool>(py))
                        .unwrap_or(false)
                })
            };
            let client = client.lock().await;
            let rx = client
                .receive_json(should_continue, &opts.unwrap_or_default())
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;

            Ok(Python::with_gil(|_py| {
                PyValueStream::new(ValueStreamer::new(rx))
            }))
        })
    }

    pub fn request_reply<'p>(
        &self,
        py: Python<'p>,
        method: String,
        payload: String,
    ) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();

        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let value: Value = serde_json::from_str(&payload)
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;
            let client = client.lock().await;
            let response = client
                .request_reply::<SimpleValue>(&method, value)
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;
            // TODO: Figure out how to marshall this as a python dict
            Ok(response.to_string())
        })
    }

    pub fn call<'p>(&self, py: Python<'p>, method: String) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();

        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let guard = client.lock_owned().await;
            let conn = crate::call(guard, &method)
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;
            Ok(SrpcMethodCallConn(Arc::new(
                Mutex::new(conn),
            )))
        })
    }
}

#[pymethods]
impl SrpcMethodCallConn {
    pub fn decode<'p>(&self, py: Python<'p>) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();

        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let mut guard = client.lock_owned().await;
            let response = guard
                .decode()
                .await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;
            Ok(response.to_string())
        })
    }

    pub fn close<'p>(&mut self, py: Python<'p>) -> PyResult<Bound<'p, PyAny>> {
        let client = self.0.clone();

        pyo3_async_runtimes::tokio::future_into_py(py, async move {
            let mut guard = client.lock_owned().await;
            guard.close();
            Ok(())
        })
    }
}

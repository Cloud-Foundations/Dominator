use crate::Client;
use pyo3::prelude::*;
use pyo3::exceptions::PyRuntimeError;
use pyo3_asyncio;
use serde_json::Value;
use std::sync::Arc;
use tokio::sync::Mutex;

#[pyclass]
pub struct SrpcClient(Arc<Mutex<Client>>);

#[pymethods]
impl SrpcClient {
    #[new]
    pub fn new(host: &str, port: u16, path: &str, cert: &str, key: &str) -> Self {
        SrpcClient(Arc::new(Mutex::new(Client::new(host, port, path, cert, key))))
    }

    pub fn connect<'p>(&self, py: Python<'p>) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        pyo3_asyncio::tokio::future_into_py(py, async move {
            client.lock().await.connect().await.map_err(|e| PyRuntimeError::new_err(e.to_string()))
        })
    }

    pub fn send_message<'p>(&self, py: Python<'p>, message: String) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        pyo3_asyncio::tokio::future_into_py(py, async move {
            client.lock().await.send_message(&message).await.map_err(|e| PyRuntimeError::new_err(e.to_string()))
        })
    }

    pub fn receive_message<'p>(&self, py: Python<'p>, expect_empty: bool) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        pyo3_asyncio::tokio::future_into_py(py, async move {
            let mut rx = client.lock().await.receive_message(expect_empty, |_| false).await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;
            
            let mut results = Vec::new();
            while let Some(result) = rx.recv().await {
                match result {
                    Ok(message) => results.push(message),
                    Err(e) => return Err(PyRuntimeError::new_err(e.to_string())),
                }
            }
            Ok(Python::with_gil(|py| results.to_object(py)))
        })
    }

    pub fn send_json<'p>(&self, py: Python<'p>, payload: String) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        pyo3_asyncio::tokio::future_into_py(py, async move {
            let value: Value = serde_json::from_str(&payload).map_err(|e| PyRuntimeError::new_err(e.to_string()))?;
            client.lock().await.send_json(&value).await.map_err(|e| PyRuntimeError::new_err(e.to_string()))
        })
    }

    pub fn receive_json<'p>(&self, py: Python<'p>) -> PyResult<&'p PyAny> {
        let client = self.0.clone();
        pyo3_asyncio::tokio::future_into_py(py, async move {
            let mut rx = client.lock().await.receive_json(|_| false).await
                .map_err(|e| PyRuntimeError::new_err(e.to_string()))?;
            
            let mut results = Vec::new();
            while let Some(result) = rx.recv().await {
                match result {
                    Ok(json_value) => results.push(json_value.to_string()),
                    Err(e) => return Err(PyRuntimeError::new_err(e.to_string())),
                }
            }
            Ok(Python::with_gil(|py| results.to_object(py)))
        })
    }
}

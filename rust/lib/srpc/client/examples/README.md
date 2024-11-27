# SRPC Client Examples

This directory contains examples for using the SRPC client in both Rust and Python.

## Rust Example

To run the Rust example:
```
cargo run --example rust_client_example --no-default-features
```

## Python Example

Requires Python >= 3.7
To run the Python example:

1. Build the Rust library with Python bindings:
```
maturin build --features python
```

2. Install the built wheel:
```
pip install target/wheels/srpc_client-*.whl
```

3. Run the Python example:
```
python3 examples/python_client_example.py
```

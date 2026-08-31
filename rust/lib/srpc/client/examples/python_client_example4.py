"""
This example demonstrates how to use the srpc_client Python bindings.

To run this example:
1. Build and install the Rust python library: maturin develop --features python
3. Run this script:
    RUST_LOG=trace \
    EXAMPLE_4_SRPC_SERVER_HOST=<host> \
    EXAMPLE_4_SRPC_SERVER_PORT=<port> \
    EXAMPLE_4_SRPC_SERVER_ENPOINT=<srpc endpoint> \
    EXAMPLE_4_SRPC_SERVER_CERT=<path to .cert> \
    EXAMPLE_4_SRPC_SERVER_KEY=<path to .key> \
    python examples/python_client_example4.py
"""

import asyncio
import json
import os
from srpc_client import SrpcClientConfig


async def main():
    print("Starting client..")

    # Create a new ClientConfig instance
    client = SrpcClientConfig(
        os.environ["EXAMPLE_4_SRPC_SERVER_HOST"],
        int(os.environ["EXAMPLE_4_SRPC_SERVER_PORT"]),
        os.environ["EXAMPLE_4_SRPC_SERVER_ENPOINT"],
        os.environ["EXAMPLE_4_SRPC_SERVER_CERT"],
        os.environ["EXAMPLE_4_SRPC_SERVER_KEY"],
    )

    # Connect to the server
    client = await client.connect()
    print("Connected to server")

    # Send a message
    message = "Hypervisor.GetUpdates\n"
    print(f"Calling server with message: {message}")
    conn = await client.call(message)
    response = await conn.decode()
    print(f"Received response: {json.loads(response)}")
    await conn.close()

    print(f"Calling server with message again: {message}")
    conn2 = await client.call(message)
    response = await conn2.decode()
    print(f"Received response: {json.loads(response)}")
    await conn2.close()


if __name__ == "__main__":
    asyncio.run(main())

"""
This example demonstrates how to use the srpc_client Python bindings.

To run this example:
1. Build and install the Rust python library: maturin develop --features python
3. Run this script:
    RUST_LOG=trace \
    EXAMPLE_2_SRPC_SERVER_HOST=<host> \
    EXAMPLE_2_SRPC_SERVER_PORT=<port> \
    EXAMPLE_2_SRPC_SERVER_ENPOINT=<srpc endpoint> \
    EXAMPLE_2_SRPC_SERVER_CERT=<path to .cert> \
    EXAMPLE_2_SRPC_SERVER_KEY=<path to .key> \
    python examples/python_client_example2.py
"""

import asyncio
import json
import os
from srpc_client import SrpcClientConfig


async def main():
    print("Starting client..")

    # Create a new ClientConfig instance
    client = SrpcClientConfig(
        os.environ["EXAMPLE_2_SRPC_SERVER_HOST"],
        int(os.environ["EXAMPLE_2_SRPC_SERVER_PORT"]),
        os.environ["EXAMPLE_2_SRPC_SERVER_ENPOINT"],
        os.environ["EXAMPLE_2_SRPC_SERVER_CERT"],
        os.environ["EXAMPLE_2_SRPC_SERVER_KEY"],
    )

    # Connect to the server
    client = await client.connect()
    print("Connected to server")

    # Send a message
    message = "Hypervisor.GetUpdates\n"
    print(f"Sending message: {message}")
    await client.send_message(message)
    print(f"Sent message: {message}")

    # Receive an empty response
    print("Waiting for empty string response...")
    responses = await client.receive_message(expect_empty=True, should_continue=False)
    async for response in responses:
        print(f"Received response: {response}")

    # Receive responses
    responses = await client.receive_json_cb(should_continue=lambda _: True)
    async for response in responses:
        print(f"Received response: {json.loads(response)}")


if __name__ == "__main__":
    asyncio.run(main())

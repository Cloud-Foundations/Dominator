"""
This example demonstrates how to use the srpc_client Python bindings.

To run this example:
1. Build the Rust library: maturin build --features python
2. Install the wheel: pip install target/wheels/srpc_client-*.whl
3. Run this script: python examples/python_client_example.py
"""

import asyncio
import json
from srpc_client import SrpcClientConfig


async def main():
    print("Starting client..")

    # Create a new ClientConfig instance
    client = SrpcClientConfig(
        "<Hostname or IP of hypervisor>",
        6976,
        "/_SRPC_/TLS/JSON",
        "<Path to Keymaster Certificate file>",
        "<Path to Keymaster Key file>",
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

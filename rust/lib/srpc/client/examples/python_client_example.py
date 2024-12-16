"""
This example demonstrates how to use the srpc_client Python bindings.

To run this example:
1. Build and install the Rust python library: maturin develop --features python
3. Run this script:
    RUST_LOG=trace \
    EXAMPLE_1_SRPC_SERVER_HOST=<host> \
    EXAMPLE_1_SRPC_SERVER_PORT=<port> \
    EXAMPLE_1_SRPC_SERVER_ENPOINT=<srpc endpoint> \
    EXAMPLE_1_SRPC_SERVER_CERT=<path to .cert> \
    EXAMPLE_1_SRPC_SERVER_KEY=<path to .key> \
    python examples/python_client_example.py
"""

import asyncio
import json
import os
from srpc_client import SrpcClientConfig


async def main():
    client = SrpcClientConfig(
        os.environ["EXAMPLE_1_SRPC_SERVER_HOST"],
        int(os.environ["EXAMPLE_1_SRPC_SERVER_PORT"]),
        os.environ["EXAMPLE_1_SRPC_SERVER_ENPOINT"],
        os.environ["EXAMPLE_1_SRPC_SERVER_CERT"],
        os.environ["EXAMPLE_1_SRPC_SERVER_KEY"],
    )

    client = await client.connect()
    print("Connected to server")

    message = "Hypervisor.StartVm\n"
    await client.send_message(message)
    print(f"Sent message: {message.strip()}")

    responses = await client.receive_message(True)
    for response in responses:
        print(f"Received response: {response}")

    json_payload = {"IpAddress": "<IP Address of VM>"}
    json_string = json.dumps(json_payload)
    await client.send_json(json_string)
    print(f"Sent JSON payload: {json_payload}")

    json_responses = await client.receive_json()
    for json_response in json_responses:
        print(f"Received JSON response: {json.loads(json_response)}")


if __name__ == "__main__":
    asyncio.run(main())

"""
This example demonstrates how to use the srpc_client Python bindings.

To run this example:
1. Build the Rust library: maturin build --features python
2. Install the wheel: pip install target/wheels/srpc_client-*.whl
3. Run this script: python examples/python_client_example.py
"""

import asyncio
import json
import os
from srpc_client import SrpcClientConfig


async def main():
    print("Starting client..")

    # Create a new ClientConfig instance
    client = SrpcClientConfig(
        os.environ["EXAMPLE_3_SRPC_SERVER_HOST"],
        int(os.environ["EXAMPLE_3_SRPC_SERVER_PORT"]),
        os.environ["EXAMPLE_3_SRPC_SERVER_ENPOINT"],
        os.environ["EXAMPLE_3_SRPC_SERVER_CERT"],
        os.environ["EXAMPLE_3_SRPC_SERVER_KEY"],
    )

    # Connect to the server
    client = await client.connect()
    print("Connected to server")

    message = "Hypervisor.ListVMs\n"

    # Send a message
    print(f"Sending message: {message}")
    await client.send_message(message)
    print(f"Sent message: {message}")

    # Receive an empty response
    print("Waiting for empty string response...")
    responses = await client.receive_message(expect_empty=True, should_continue=False)
    async for response in responses:
        print(f"Received response: {response}")

    # Send a JSON message
    payload = json.dumps(
        {
            "IgnoreStateMask": 0,
            "OwnerGroups": [],
            "OwnerUsers": [],
            "Sort": True,
            "VmTagsToMatch": {},
        }
    )
    print(f"Sending payload: {payload}")
    await client.send_json(payload)
    print(f"Sent payload: {payload}")

    # Receive an empty response
    print("Waiting for empty string response for payload...")
    responses = await client.receive_message(expect_empty=True, should_continue=False)
    async for response in responses:
        print(f"Received response: {response}")

    # Receive responses
    print("Waiting for response...")
    responses = await client.receive_json_cb(should_continue=lambda _: False)
    async for response in responses:
        print(f"Received response: {json.loads(response)}")

    # Use RequestReply
    print(f"Sending request_reply: {message}")
    res = await client.request_reply(
        message,
        json.dumps(
            {
                "IgnoreStateMask": 0,
                "OwnerGroups": [],
                "OwnerUsers": [],
                "Sort": True,
                "VmTagsToMatch": {},
            }
        ),
    )
    print(f"Sent request_reply: {message}, got reply: {res}")


if __name__ == "__main__":
    asyncio.run(main())

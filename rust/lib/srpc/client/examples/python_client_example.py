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
    client = SrpcClientConfig("<Hostname or IP of hypervisor>", 6976, "/_SRPC_/TLS/JSON", "<Path to Keymaster Certificate file>", "<Path to Keymaster Key file>")

    client = await client.connect()
    print("Connected to server")

    message = "Hypervisor.StartVm\n"
    await client.send_message(message)
    print(f"Sent message: {message.strip()}")

    responses = await client.receive_message(True)
    for response in responses:
        print(f"Received response: {response}")

    json_payload = {
        "IpAddress": "<IP Address of VM>"
    }
    json_string = json.dumps(json_payload)
    await client.send_json(json_string)
    print(f"Sent JSON payload: {json_payload}")

    json_responses = await client.receive_json()
    for json_response in json_responses:
        print(f"Received JSON response: {json.loads(json_response)}")

if __name__ == "__main__":
    asyncio.run(main())

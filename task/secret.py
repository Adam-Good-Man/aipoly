from py_clob_client.client import ClobClient
import os


client = ClobClient(
    host="https://clob.polymarket.com",
    chain_id=137,
    key="165fdc94f51393f872962419352466032ff33b3a87e362842f1833cc5f54bf9b"
)


creds = client.create_or_derive_api_creds()
print(creds)
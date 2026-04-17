# eka-usage-sdk (Python)

Non-blocking SDK for pushing usage metrics to Kafka.

One client per process, many workspaces per client.

## Install

```bash
pip install git+ssh://git@github.com/eka-care/eka-usage-sdk.git#subdirectory=sdks/python
```

## Configure

Kafka connection and tuning come from environment variables:

| Env var                      | Default | Kafka key          |
| ---------------------------- | ------- | ------------------ |
| `EKA_KAFKA_BROKERS`          | —       | `bootstrap.servers` |
| `EKA_KAFKA_LINGER_MS`        | `50`    | `linger.ms`         |
| `EKA_KAFKA_BATCH_SIZE`       | `65536` | `batch.size`        |
| `EKA_KAFKA_COMPRESSION_TYPE` | `lz4`   | `compression.type`  |
| `EKA_KAFKA_ACKS`             | `1`     | `acks`              |
| `EKA_KAFKA_RETRIES`          | `5`     | `retries`           |

`EKA_KAFKA_BROKERS` is required unless you pass `kafka_brokers=` to the constructor.

## Usage

```python
from eka_usage import EkaClient

client = EkaClient(
    service_name="scribe-api",
    on_error=lambda e, ctx: print("sdk err", e),
)

# workspace_id is per-call — one client serves all tenants
client.record("ws_123", "ekascribe", "transcription_minute", quantity=8.2)

# with unit_cost and error status (metadata becomes the error log)
client.record("ws_123", "api", "api_error", status="error",
              metadata={"error": "upstream timeout", "endpoint": "/v1/records"})

client.shutdown()
```

Or as a context manager:

```python
with EkaClient(service_name="svc") as c:
    c.record("ws_123", "api", "api_call", unit_cost=0.05)
```

## Tests

```bash
pip install -e ".[dev]"
pytest
```

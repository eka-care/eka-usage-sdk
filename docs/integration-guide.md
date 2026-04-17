# Integration guide

The Eka Usage SDK ships metered usage events to Kafka on a single topic:
`eka.usage.events`. When `status="error"`, the metadata JSON serves as the
error log — no separate logging method or topic needed.

Every SDK exposes the same three-method surface:
`EkaClient(...)`, `record(...)`, `shutdown()`.

The SDKs are non-blocking: `record` returns immediately. Kafka I/O happens on
a background worker. Errors never propagate to the caller — they go to the
`on_error` callback you supply.

## Golden rules

1. Create one `EkaClient` per process, not per request.
2. Call `shutdown()` on SIGTERM so in-flight events flush before the process exits.
3. Pass an `on_error` callback in production. You want visibility into validation
   failures and Kafka outages — don't let them vanish.
4. Never block the hot path. If you're tempted to `await` a `record` call,
   you're using the wrong API.

---

## Python

One client per process serves all tenants — pass `workspace_id` on each call.

Kafka connection and tuning come from env vars (`EKA_KAFKA_BROKERS`,
`EKA_KAFKA_LINGER_MS`, `EKA_KAFKA_BATCH_SIZE`, `EKA_KAFKA_COMPRESSION_TYPE`,
`EKA_KAFKA_ACKS`, `EKA_KAFKA_RETRIES`). Explicit constructor args override env.

```python
from eka_usage import EkaClient

client = EkaClient(
    service_name="scribe-api",
    on_error=lambda e, ctx: logger.error("eka-sdk: %s %s", e, ctx),
)

# Metered usage
client.record("ws_123", "ekascribe", "transcription_minute", quantity=8.2,
              metadata={"patient_id": "p_abc"})

# Error event — metadata is the error log
client.record("ws_123", "api", "api_error", status="error",
              metadata={"error": "upstream timeout", "endpoint": "/v1/records"})

# On shutdown (e.g. in an atexit or signal handler)
client.shutdown()
```

Or as a context manager:

```python
with EkaClient(service_name="svc") as c:
    c.record("ws_123", "api", "api_call", unit_cost=0.05)
```

## TypeScript / Node.js

One client per process serves all tenants — pass `workspaceId` on each call.

Kafka connection from env: `EKA_KAFKA_BROKERS`, `EKA_KAFKA_COMPRESSION_TYPE`,
`EKA_KAFKA_ACKS`, `EKA_KAFKA_RETRIES`. Explicit constructor args override env.
(`linger.ms`/`batch.size` are librdkafka-only; use `flushIntervalMs`/`maxQueueSize` for TS.)

```ts
import { EkaClient } from "@eka-care/usage-sdk";

const client = new EkaClient({
  serviceName: "scribe-api",
  onError: (e, ctx) => logger.error({ err: e, ctx }, "eka-sdk"),
});

await client.connect();  // required once on startup

client.record("ws_123", "ekascribe", "transcription_minute", 8.2);

// error event — metadata is the error log
client.record("ws_123", "api", "api_error", 1, "error", undefined,
              { error: "upstream timeout", endpoint: "/v1/records" });

process.on("SIGTERM", () => client.shutdown());
```

## Go

One client per process serves all tenants — pass `workspaceID` on each call.

Kafka connection and tuning from env: `EKA_KAFKA_BROKERS`,
`EKA_KAFKA_LINGER_MS`, `EKA_KAFKA_BATCH_SIZE`, `EKA_KAFKA_COMPRESSION_TYPE`,
`EKA_KAFKA_ACKS`, `EKA_KAFKA_RETRIES`. Explicit options override env.

```go
client, err := ekausage.New(
    "scribe-api",
    ekausage.WithOnError(func(e error, ctx map[string]any) {
        log.Printf("eka-sdk: %v %v", e, ctx)
    }),
)
if err != nil {
    log.Fatal(err)
}
defer client.Shutdown()

client.Record("ws_123", "ekascribe", "transcription_minute", 8.2, "ok", nil,
    map[string]any{"patient_id": "p_abc"})

// error event — metadata is the error log
client.Record("ws_123", "api", "api_error", 1, "error", nil,
    map[string]any{"error": "upstream timeout"})
```

Build with `-tags confluent` to link the real Kafka producer. Without the tag,
the Go SDK compiles cleanly but expects you to pass a `Producer` via
`WithProducer(...)` — handy for tests.

---

## Operations

- **Topic setup.** Run `infra/create-topics.sh` once per Kafka cluster. It is
  idempotent.
- **ClickHouse sink.** `infra/kafka-connect-sink.json` is a ready-to-POST
  Kafka Connect config. The SDK has zero knowledge of ClickHouse — the sink
  is the only component that knows the schema downstream.
- **Backpressure.** Each SDK uses a bounded in-memory buffer. If Kafka is
  down for long enough to fill it, new events are dropped and `on_error` fires
  with `reason=local_queue_full`. This is intentional — we favor keeping the
  host application healthy over preserving every event.
- **Partitioning.** `workspace_id` is the partition key, which keeps
  per-workspace events in order and gives the sink good write locality.

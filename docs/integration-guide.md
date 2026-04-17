# Integration guide

The Eka Usage SDK ships two streams to Kafka:

- `eka.usage.events` — metered product usage (billable and non-billable)
- `eka.service.logs` — critical application logs

Every SDK exposes the same four-method surface:
`EkaClient(...)`, `record(...)`, `log(...)`, `shutdown()`.

The SDKs are non-blocking: `record` and `log` return immediately. Kafka I/O
happens on a background worker. Errors never propagate to the caller — they go
to the `on_error` callback you supply.

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

# Critical log
client.log("ws_123", "error", "ffmpeg failed", code="FFMPEG_EXIT_137",
           metadata={"file_id": "f_42"})

# On shutdown (e.g. in an atexit or signal handler)
client.shutdown()
```

Or as a context manager:

```python
with EkaClient(service_name="svc") as c:
    c.record("ws_123", "api", "api_call")
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

client.record("ws_123", "ekascribe", "transcription_minute", 8.2, "ok",
              { patient_id: "p_abc" });
client.log("ws_123", "error", "ffmpeg failed", "FFMPEG_EXIT_137",
           { file_id: "f_42" });

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

client.Record("ws_123", "ekascribe", "transcription_minute", 8.2, "ok",
    map[string]any{"patient_id": "p_abc"})
client.Log("ws_123", "error", "ffmpeg failed", "FFMPEG_EXIT_137",
    map[string]any{"file_id": "f_42"})
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
- **Partitioning.** `workspace_id` is the partition key on both topics, which
  keeps per-workspace events in order and gives the sink good write locality.

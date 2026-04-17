# @eka-care/usage-sdk (TypeScript / Node.js)

Non-blocking SDK for pushing usage metrics and critical logs to Kafka.

One client per process, many workspaces per client.

## Install

```bash
npm install @eka-care/usage-sdk
```

## Configure

Kafka connection and tuning come from environment variables:

| Env var                      | Default | Applies to             |
| ---------------------------- | ------- | ---------------------- |
| `EKA_KAFKA_BROKERS`          | —       | broker list (required) |
| `EKA_KAFKA_COMPRESSION_TYPE` | `lz4`   | `lz4`/`gzip`/`snappy`/`zstd`/`none` |
| `EKA_KAFKA_ACKS`             | `1`     | `-1` (or `all`), `0`, `1` |
| `EKA_KAFKA_RETRIES`          | `5`     | producer retry count   |

`linger.ms` and `batch.size` are librdkafka-only and do not apply to kafkajs.
Use `flushIntervalMs` and `maxQueueSize` options for TS-side batching tuning.

## Usage

```ts
import { EkaClient } from "@eka-care/usage-sdk";

const client = new EkaClient({
  serviceName: "scribe-api",
  onError: (e, ctx) => console.error("sdk err", e.message, ctx),
});

await client.connect();

// workspaceId is per-call — one client serves all tenants
client.record("ws_123", "ekascribe", "transcription_minute", 8.2);
client.log("ws_123", "error", "ffmpeg failed", "FFMPEG_EXIT_137");

process.on("SIGTERM", () => client.shutdown());
```

Explicit options override env:

```ts
new EkaClient({
  serviceName: "svc",
  kafkaBrokers: "kafka-1:9092,kafka-2:9092",
  compression: "zstd",
  acks: -1,
  retries: 10,
});
```

## Tests

```bash
npm install
npm test
```

## Build

```bash
npm run build
```

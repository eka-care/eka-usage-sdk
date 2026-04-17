# eka-usage-sdk (Go)

Non-blocking SDK for pushing usage metrics and critical logs to Kafka.

One client per process, many workspaces per client.

## Install

```bash
go get github.com/eka-care/eka-usage-sdk/sdks/go
```

For the default confluent-kafka-go producer, also fetch:

```bash
go get github.com/confluentinc/confluent-kafka-go/v2
```

and build your binary with `-tags confluent`.

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

`EKA_KAFKA_BROKERS` is required unless you pass `WithKafkaBrokers(...)`.

## Usage

```go
client, err := ekausage.New(
    "scribe-api",
    ekausage.WithOnError(func(e error, ctx map[string]any) {
        log.Printf("sdk err: %v", e)
    }),
)
if err != nil { log.Fatal(err) }
defer client.Shutdown()

// workspaceID is per-call — one client serves all tenants
client.Record("ws_123", "ekascribe", "transcription_minute", 8.2, "ok", nil)
client.Log("ws_123", "error", "ffmpeg failed", "FFMPEG_EXIT_137", nil)
```

Explicit options override env:

```go
ekausage.New("svc",
    ekausage.WithKafkaBrokers("kafka-1:9092,kafka-2:9092"),
    ekausage.WithKafkaConfig(map[string]any{"linger.ms": 100}),
)
```

## Tests

```bash
go test ./...
```

Tests use a mock producer and do not require the confluent-kafka-go dependency.

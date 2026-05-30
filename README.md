# eka-usage-sdk

Monorepo of language-specific SDKs that push metered usage and critical logs
from Eka Care services to Kafka.

## Install

**Python** (pip / uv):
```bash
pip install git+https://github.com/eka-care/eka-usage-sdk.git#subdirectory=sdks/python
```
Or in `pyproject.toml`:
```toml
dependencies = ["eka-usage-sdk @ git+https://github.com/eka-care/eka-usage-sdk.git#subdirectory=sdks/python"]
```

**Go**:
```bash
go get github.com/eka-care/eka-usage-sdk/sdks/go
```
Build with `-tags confluent` to link the real Kafka producer.

**TypeScript / Node.js** (published to GitHub Packages on `ts-v*` tags):
```bash
npm install @eka-care/usage-sdk
```

## Configuration

Kafka connection and tuning come from environment variables — no hardcoded
broker addresses in service code.

| Env var                      | Default | Notes                              |
| ---------------------------- | ------- | ---------------------------------- |
| `EKA_KAFKA_BROKERS`          | —       | Required                           |
| `EKA_KAFKA_LINGER_MS`        | `50`    | Python / Go only (librdkafka)      |
| `EKA_KAFKA_BATCH_SIZE`       | `65536` | Python / Go only (librdkafka)      |
| `EKA_KAFKA_COMPRESSION_TYPE` | `lz4`   | All SDKs                           |
| `EKA_KAFKA_ACKS`             | `1`     | All SDKs                           |
| `EKA_KAFKA_RETRIES`          | `5`     | All SDKs                           |

Explicit constructor args always override env.

## API surface

All three SDKs expose the same three-method surface. One client per process
serves all tenants — `workspace_id` is passed per call.

```
EkaClient(service_name, ...)
client.record(workspace_id, product, metric_type, quantity=1.0, status="ok", unit_cost=None, metadata={}, idp=None, c_id=None)
client.shutdown()
```

When `status="error"`, the `metadata` JSON string serves as the error log.

### Billing tags (`idp` / `c_id`)

Two optional per-call fields carry the JWT-derived billing classification.
The calling service extracts them from the request's JWT (the SDK does no
token parsing or I/O of its own) and passes them through:

- `idp` — the JWT `idp` claim. Drives `is_billable` (see below).
- `c_id` — the JWT `c-id` claim (the API key identifier). Copied verbatim
  into the event as `c_id` for downstream per-API-key analytics; it does not
  affect billing. Defaults to `""` when absent.

Go passes these via `RecordOption`s: `WithIDP("api-key")`, `WithCID("ak_...")`.
TypeScript takes them as the trailing `idp` / `cId` arguments.

Events emitted with no caller context (background jobs, scheduled tasks) simply
omit `idp`/`c_id`, so they emit `is_billable=0` and `c_id=""`.

## Products and metric types

| Product     | Metric types                                                                                      |
|-------------|---------------------------------------------------------------------------------------------------|
| `ekascribe` | `transcription_minute`, `transcription_session`                                                   |
| `mr_ai`     | `mr_record_upload`, `mr_page_processed`                                                           |
| `agent`     | `chat_session`, `tool_call`, `input_token`, `output_token`, `message`                             |
| `api`       | `api_call`, `api_error`                                                                           |
| `webhooks`  | `webhook_push`, `webhook_delivery_failed`                                                         |
| `emr_tools` | `tool_call`                                                                                       |
| `clinical_tools` | `tool_call`                                                                                  |
| `comms`     | `sms`, `whatsapp`, `email`                                                                        |
| `abdm`      | `abha`, `linking`, `data_transfer`                                                                |

**Statuses:** `ok` or `error`. When `status="error"`, the `metadata` JSON string serves as the error log.

The SDK validates products, metric types, and statuses at call time — invalid values go to `on_error`, never to Kafka.

**`is_billable`** is set to `1` only when the event is a successful outcome **and** it
originates from an API caller — i.e. `status == "ok"` **and** `idp == "api-key"`.
Every other case (error outcomes, SaaS/subscription/internal callers, or no caller
context) emits `is_billable=0`, and the downstream reconciler skips it.

All events flow through a single Kafka topic:

| Topic              | Partitions | Retention | Key            |
|--------------------|-----------:|----------:|----------------|
| `eka.usage.events` |         10 |     7 d   | `workspace_id` |

A Kafka Connect ClickHouse sink (`infra/kafka-connect-sink.json`) consumes
the topic downstream. The SDK has no knowledge of ClickHouse.

## Principles

- **Never block the caller.** `record` returns immediately.
- **Never crash the host.** All Kafka errors are caught and routed to `on_error`.
- **Never require connection management.** The SDK owns the producer lifecycle.
- **Always enrich server-side fields.** `ts`, `sdk_version`, and `hostname`
  are set inside the SDK so callers cannot spoof them.
- **Always validate before producing.** Invalid events never reach Kafka —
  they go to `on_error`.

## Layout

```
eka-usage-sdk/
├── README.md
├── Makefile                       # test-all / lint-all / build-all
├── .github/workflows/
│   └── publish-ts.yml             # publish TS SDK to GitHub Packages
├── infra/
│   ├── create-topics.sh           # one-shot Kafka topic setup
│   └── kafka-connect-sink.json    # ClickHouse sink config
├── sdks/
│   ├── python/
│   ├── typescript/
│   └── go/
└── docs/
    ├── integration-guide.md
    └── metric-types.md
```

## Quick start

```bash
# Run every language's tests (uses mock Kafka producers)
make test-all

# Lint everything
make lint-all

# Build every artifact
make build-all
```

Per-SDK READMEs under `sdks/<lang>/README.md` cover language-specific install
and usage. The canonical list of valid products, metric types, statuses, and
log levels lives in `docs/metric-types.md` — changes there must be mirrored
in all three SDK constants files.

## Publishing

- **Python / Go**: Services install directly from this repo (no publish step).
- **TypeScript**: Push a tag like `ts-v0.2.0` to trigger the GitHub Actions
  workflow that publishes to GitHub Packages. Bump `version` in
  `sdks/typescript/package.json` before tagging.

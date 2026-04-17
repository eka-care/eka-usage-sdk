# Valid products, metric types, statuses, and log levels

This list is the authoritative contract between SDKs and the consumer side.
Adding a product or metric type requires a change in all three SDKs
(`sdks/python/eka_usage/constants.py`, `sdks/typescript/src/constants.ts`,
`sdks/go/ekausage/constants.go`).

## Products and metric types

| Product     | Metric types                                                              |
|-------------|---------------------------------------------------------------------------|
| `ekascribe` | `transcription_minute`, `transcription_session`                           |
| `mr_ai`     | `mr_record_upload`, `mr_page_processed`                                   |
| `agent`     | `chat_session`, `tool_call`, `tool_call_error`, `credit_consumed`         |
| `api`       | `api_call`, `api_error`                                                   |
| `webhooks`  | `webhook_push`, `webhook_delivery_failed`                                 |

## Statuses

`ok` — billable outcome (`is_billable = 1`).
`error` — non-billable outcome (`is_billable = 0`).

## Log levels

`error`, `critical`, `warning`.

The SDK enforces these values. Invalid inputs do not crash the caller — they
are routed to `on_error` and never sent to Kafka.

## Message schemas

### `eka.usage.events`

```json
{
  "workspace_id": "ws_123",
  "service_name": "scribe-api",
  "product": "ekascribe",
  "metric_type": "transcription_minute",
  "quantity": 8.2,
  "status": "ok",
  "is_billable": 1,
  "metadata": "{\"patient_id\":\"p_abc\"}",
  "sdk_language": "python",
  "sdk_version": "0.1.0",
  "hostname": "scribe-api-7b9c",
  "ts": "2026-04-16T09:12:33.481Z"
}
```

### `eka.service.logs`

```json
{
  "workspace_id": "ws_123",
  "service_name": "scribe-api",
  "level": "error",
  "message": "ffmpeg failed",
  "code": "FFMPEG_EXIT_137",
  "metadata": "{\"file_id\":\"f_42\"}",
  "sdk_language": "python",
  "sdk_version": "0.1.0",
  "hostname": "scribe-api-7b9c",
  "ts": "2026-04-16T09:12:33.481Z"
}
```

`metadata` is always a JSON-encoded string, not a nested object. This keeps the
ClickHouse schema stable when callers attach arbitrary fields.

# Valid products, metric types, and statuses

This list is the authoritative contract between SDKs and the consumer side.
Adding a product or metric type requires a change in all three SDKs
(`sdks/python/eka_usage/constants.py`, `sdks/typescript/src/constants.ts`,
`sdks/go/ekausage/constants.go`).

## Products and metric types

| Product     | Metric types                                                              |
|-------------|---------------------------------------------------------------------------|
| `ekascribe` | `transcription_minute`, `transcription_session`                           |
| `mr_ai`     | `mr_record_upload`, `mr_page_processed`                                   |
| `agent`     | `chat_session`, `tool_call`, `input_token`, `output_token`, `message`     |
| `api`       | `api_call`, `api_error`                                                   |
| `webhooks`  | `webhook_push`, `webhook_delivery_failed`                                 |
| `emr_tools` | `tool_call`                                                              |
| `clinical_tools` | `tool_call`                                                         |
| `comms`     | `sms`, `whatsapp`, `email`                                                |
| `abdm`      | `abha`, `linking`, `data_transfer`                                        |

There is no dedicated error metric type. A failed tool call (or any failed
outcome) is recorded as its normal metric type with `status="error"` — the
error is picked up from `status`, not from a separate `*_error` metric.

## Statuses

`ok` — successful outcome.
`error` — failed outcome. Metadata JSON serves as the error log.

The SDK enforces these values. Invalid inputs do not crash the caller — they
are routed to `on_error` and never sent to Kafka.

## Billing classification (`is_billable`, `c_id`)

`is_billable` (`0`/`1`) tells the downstream reconciler whether to deduct credit
for the event. It is `1` only when the outcome succeeded **and** the caller is an
API key:

```
is_billable = 1  iff  status == "ok"  and  idp == "api-key"
```

`idp` and `c_id` come from the request JWT (`idp` and `c-id` claims). The calling
service extracts them and passes them per call — the SDK does no token parsing.
`idp` is never written to the event; only the derived `is_billable` is. `c_id`
(API key identifier) is copied verbatim for per-API-key analytics and does not
affect billing; it defaults to `""`.

> The ClickHouse `is_billable` column defaults to `1` and `c_id` to `''`. That
> default only applies to rows inserted **without** these fields — i.e. when a
> producer bypasses this SDK. Events emitted through the SDK always carry both
> fields explicitly (no-caller-context events emit `is_billable=0`, `c_id=""`).

## Message schema (`eka.usage.events`)

```json
{
  "workspace_id": "ws_123",
  "service_name": "scribe-api",
  "product": "ekascribe",
  "metric_type": "transcription_minute",
  "quantity": 8.2,
  "unit_cost": null,
  "status": "ok",
  "is_billable": 1,
  "c_id": "ak_a3f2b9c1",
  "metadata": "{\"patient_id\":\"p_abc\"}",
  "sdk_language": "python",
  "sdk_version": "0.2.0",
  "hostname": "scribe-api-7b9c",
  "ts": "2026-04-16T09:12:33.481Z"
}
```

`metadata` is always a JSON-encoded string, not a nested object. This keeps the
ClickHouse schema stable when callers attach arbitrary fields.

`unit_cost` is nullable — omit it when cost is computed downstream.

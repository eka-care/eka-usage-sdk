export const SDK_VERSION = "0.1.0";
export const SDK_LANGUAGE = "typescript";

export const USAGE_TOPIC = "eka.usage.events";
export const USAGE_DLQ = "eka.usage.events.dlq";

export const ENV_KAFKA_BROKERS = "EKA_KAFKA_BROKERS";
export const ENV_KAFKA_COMPRESSION_TYPE = "EKA_KAFKA_COMPRESSION_TYPE";
export const ENV_KAFKA_ACKS = "EKA_KAFKA_ACKS";
export const ENV_KAFKA_RETRIES = "EKA_KAFKA_RETRIES";

export const PRODUCTS = [
  "ekascribe",
  "mr_ai",
  "agent",
  "api",
  "webhooks",
] as const;
export type Product = (typeof PRODUCTS)[number];

export const METRIC_TYPES: Record<Product, readonly string[]> = {
  ekascribe: ["transcription_minute", "transcription_session"],
  mr_ai: ["mr_record_upload", "mr_page_processed"],
  agent: ["chat_session", "tool_call", "tool_call_error", "credit_consumed", "input_token", "output_token"],
  api: ["api_call", "api_error"],
  webhooks: ["webhook_push", "webhook_delivery_failed"],
};

export const STATUSES = ["ok", "error"] as const;
export type Status = (typeof STATUSES)[number];


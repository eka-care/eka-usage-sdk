SDK_VERSION = "0.1.0"
SDK_LANGUAGE = "python"

USAGE_TOPIC = "eka.usage.events"
USAGE_DLQ = "eka.usage.events.dlq"

ENV_KAFKA_BROKERS = "EKA_KAFKA_BROKERS"
ENV_KAFKA_LINGER_MS = "EKA_KAFKA_LINGER_MS"
ENV_KAFKA_BATCH_SIZE = "EKA_KAFKA_BATCH_SIZE"
ENV_KAFKA_COMPRESSION_TYPE = "EKA_KAFKA_COMPRESSION_TYPE"
ENV_KAFKA_ACKS = "EKA_KAFKA_ACKS"
ENV_KAFKA_RETRIES = "EKA_KAFKA_RETRIES"

DEFAULT_LINGER_MS = 50
DEFAULT_BATCH_SIZE = 65536
DEFAULT_COMPRESSION_TYPE = "lz4"
DEFAULT_ACKS = "1"
DEFAULT_RETRIES = 5

PRODUCTS = ("ekascribe", "mr_ai", "agent", "api", "webhooks", "emr_tools", "clinical_tools", "comms", "abdm")

METRIC_TYPES = {
    "ekascribe": ("transcription_minute", "transcription_session"),
    "mr_ai": ("mr_record_upload", "mr_page_processed"),
    "agent": ("chat_session", "tool_call", "tool_call_error", "credit_consumed", "input_token", "output_token"),
    "api": ("api_call", "api_error"),
    "webhooks": ("webhook_push", "webhook_delivery_failed"),
    "emr_tools": ("tool_call", "tool_call_error"),
    "clinical_tools": ("tool_call", "tool_call_error"),
    "comms": ("sms", "whatsapp", "email"),
    "abdm": ("abha", "linking", "data_transfer"),
}

STATUSES = ("ok", "error")

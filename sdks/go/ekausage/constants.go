package ekausage

const (
	SDKVersion  = "0.1.0"
	SDKLanguage = "go"

	UsageTopic = "eka.usage.events"
	UsageDLQ   = "eka.usage.events.dlq"

	EnvKafkaBrokers         = "EKA_KAFKA_BROKERS"
	EnvKafkaLingerMs        = "EKA_KAFKA_LINGER_MS"
	EnvKafkaBatchSize       = "EKA_KAFKA_BATCH_SIZE"
	EnvKafkaCompressionType = "EKA_KAFKA_COMPRESSION_TYPE"
	EnvKafkaAcks            = "EKA_KAFKA_ACKS"
	EnvKafkaRetries         = "EKA_KAFKA_RETRIES"

	DefaultLingerMs        = 50
	DefaultBatchSize       = 65536
	DefaultCompressionType = "lz4"
	DefaultAcks            = "1"
	DefaultRetries         = 5
)

var Products = []string{"ekascribe", "mr_ai", "agent", "api", "webhooks"}

var MetricTypes = map[string][]string{
	"ekascribe": {"transcription_minute", "transcription_session"},
	"mr_ai":     {"mr_record_upload", "mr_page_processed"},
	"agent":     {"chat_session", "tool_call", "tool_call_error", "credit_consumed"},
	"api":       {"api_call", "api_error"},
	"webhooks":  {"webhook_push", "webhook_delivery_failed"},
}

var Statuses = []string{"ok", "error"}

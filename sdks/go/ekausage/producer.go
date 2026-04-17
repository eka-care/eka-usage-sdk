package ekausage

// Producer is the minimal interface the SDK needs from a Kafka producer.
// The default implementation wraps confluent-kafka-go; tests can substitute
// a mock via Config.Producer.
type Producer interface {
	Produce(topic string, key, value []byte) error
	Flush(timeoutMs int) int
	Close()
}

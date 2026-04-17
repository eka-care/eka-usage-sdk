//go:build confluent

package ekausage

import (
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type confluentProducer struct {
	p         *kafka.Producer
	onErr     func(error, map[string]any)
	closeDone chan struct{}
}

func newConfluentProducer(brokers string, extra map[string]any, clientID string, onErr func(error, map[string]any)) (Producer, error) {
	cfg := &kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"client.id":         clientID,
		"enable.idempotence": false,
	}
	for k, v := range extra {
		_ = cfg.SetKey(k, v)
	}
	p, err := kafka.NewProducer(cfg)
	if err != nil {
		return nil, fmt.Errorf("kafka producer init: %w", err)
	}
	cp := &confluentProducer{p: p, onErr: onErr, closeDone: make(chan struct{})}
	go cp.eventLoop()
	return cp, nil
}

func (c *confluentProducer) eventLoop() {
	defer close(c.closeDone)
	for e := range c.p.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil && c.onErr != nil {
				topic := ""
				if ev.TopicPartition.Topic != nil {
					topic = *ev.TopicPartition.Topic
				}
				c.onErr(ev.TopicPartition.Error, map[string]any{"topic": topic})
			}
		case kafka.Error:
			if c.onErr != nil {
				c.onErr(ev, map[string]any{"phase": "kafka_error"})
			}
		}
	}
}

func (c *confluentProducer) Produce(topic string, key, value []byte) error {
	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            key,
		Value:          value,
	}
	return c.p.Produce(msg, nil)
}

func (c *confluentProducer) Flush(timeoutMs int) int { return c.p.Flush(timeoutMs) }

func (c *confluentProducer) Close() {
	c.p.Close()
	<-c.closeDone
}

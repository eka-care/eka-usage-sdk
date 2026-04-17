//go:build !confluent

package ekausage

import "errors"

func newConfluentProducer(brokers string, extra map[string]any, clientID string, onErr func(error, map[string]any)) (Producer, error) {
	return nil, errors.New("confluent-kafka-go not linked; build with -tags confluent or pass WithProducer(...)")
}

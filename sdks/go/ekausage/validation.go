package ekausage

import (
	"errors"
	"fmt"
	"strings"
)

type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func validateUsage(product, metricType string, quantity float64, status string) error {
	if !contains(Products, product) {
		return &ValidationError{Msg: fmt.Sprintf("invalid product '%s', allowed=[%s]", product, strings.Join(Products, ","))}
	}
	allowed, ok := MetricTypes[product]
	if !ok || !contains(allowed, metricType) {
		return &ValidationError{Msg: fmt.Sprintf("invalid metric_type '%s' for product '%s'", metricType, product)}
	}
	if !contains(Statuses, status) {
		return &ValidationError{Msg: fmt.Sprintf("invalid status '%s'", status)}
	}
	if quantity < 0 {
		return &ValidationError{Msg: "quantity must be non-negative"}
	}
	return nil
}

var ErrBufferFull = errors.New("buffer full")
var ErrClosed = errors.New("client closed")

#!/usr/bin/env bash
# Create Kafka topics required by the Eka Usage SDK.
#
# Usage:
#   BOOTSTRAP=kafka-1:9092,kafka-2:9092 ./create-topics.sh
#
# Requires: kafka-topics.sh on PATH (from the Kafka distribution).

set -euo pipefail

BOOTSTRAP="${BOOTSTRAP:-localhost:9092}"
RF="${REPLICATION_FACTOR:-3}"

create_topic() {
  local name="$1"
  local partitions="$2"
  local retention_ms="$3"

  echo ">> creating $name (partitions=$partitions, retention_ms=$retention_ms)"
  kafka-topics.sh --bootstrap-server "$BOOTSTRAP" \
    --create --if-not-exists \
    --topic "$name" \
    --partitions "$partitions" \
    --replication-factor "$RF" \
    --config retention.ms="$retention_ms" \
    --config cleanup.policy=delete \
    --config compression.type=producer
}

# Retention: 7 days for usage, 30 days for logs
SEVEN_DAYS_MS=$((7 * 24 * 60 * 60 * 1000))
THIRTY_DAYS_MS=$((30 * 24 * 60 * 60 * 1000))

create_topic "eka.usage.events"     10 "$SEVEN_DAYS_MS"
create_topic "eka.service.logs"      5 "$THIRTY_DAYS_MS"
create_topic "eka.usage.events.dlq"  3 "$THIRTY_DAYS_MS"
create_topic "eka.service.logs.dlq"  3 "$THIRTY_DAYS_MS"

echo ">> done"

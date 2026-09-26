#!/bin/bash
# Creates the initial topics and their DLQs (specs/data/kafka.md).
set -euo pipefail
BOOTSTRAP="${KAFKA_BOOTSTRAP:-kafka:9092}"
TOPICS=(catalog.events auth.events profile.events library.events playback.events playlist.events media.events)

for topic in "${TOPICS[@]}"; do
  for name in "$topic" "$topic.dlq"; do
    /opt/kafka/bin/kafka-topics.sh --bootstrap-server "$BOOTSTRAP" \
      --create --if-not-exists --topic "$name" --partitions 3 --replication-factor 1
  done
done
/opt/kafka/bin/kafka-topics.sh --bootstrap-server "$BOOTSTRAP" --list

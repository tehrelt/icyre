-- Raw playback events (playback.events), one row per Kafka event.
-- ReplacingMergeTree collapses redelivered events (same event_id) on merge;
-- readers that need exact counts use FINAL or count distinct event_id.
-- insert_deduplication_token makes a retried batch a no-op; plain MergeTree
-- needs non_replicated_deduplication_window for that.
-- Retention: raw rows live 180 days; monthly partitions let TTL drop whole
-- parts instead of rewriting them.
CREATE TABLE IF NOT EXISTS playback_events (
    event_id    UUID,
    event_type  LowCardinality(String),
    playback_id UUID,
    user_id     UUID,
    track_id    UUID,
    album_id    UUID,
    artist_ids  Array(UUID),
    source      LowCardinality(String),
    duration_ms UInt32,
    listened_ms UInt32,
    at          DateTime64(3, 'UTC'),
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toYYYYMM(at)
ORDER BY (toDate(at), track_id, event_id)
TTL toDateTime(at) + INTERVAL 180 DAY DELETE
SETTINGS ttl_only_drop_parts = 1, non_replicated_deduplication_window = 1000;

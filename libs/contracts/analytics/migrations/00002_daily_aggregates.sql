-- Daily aggregates, recomputed per day from playback_events by the analytics
-- worker (INSERT ... SELECT). Recomputing a day inserts a newer computed_at
-- and ReplacingMergeTree keeps the latest row per key, so jobs are idempotent
-- and late events are picked up by recomputing the day again.
-- Readers use FINAL (or argMax by computed_at) until parts are merged.
-- plays: playback.started; completions: playback.finished;
-- skips: playback.skipped; completion rate = completions / (completions + skips).
-- Retention: aggregates outlive raw events — 5 years.
CREATE TABLE IF NOT EXISTS daily_track_stats (
    day              Date,
    track_id         UUID,
    plays            UInt64,
    completions      UInt64,
    skips            UInt64,
    unique_listeners UInt64,
    listened_ms      UInt64,
    computed_at      DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYYYYMM(day)
ORDER BY (day, track_id)
TTL day + INTERVAL 5 YEAR DELETE
SETTINGS ttl_only_drop_parts = 1;

-- A track with several artists counts once for each of them.
CREATE TABLE IF NOT EXISTS daily_artist_stats (
    day              Date,
    artist_id        UUID,
    plays            UInt64,
    completions      UInt64,
    skips            UInt64,
    unique_listeners UInt64,
    listened_ms      UInt64,
    computed_at      DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYYYYMM(day)
ORDER BY (day, artist_id)
TTL day + INTERVAL 5 YEAR DELETE
SETTINGS ttl_only_drop_parts = 1;

-- Platform-wide totals per day; active_listeners is daily active listeners.
CREATE TABLE IF NOT EXISTS daily_totals (
    day              Date,
    plays            UInt64,
    completions      UInt64,
    skips            UInt64,
    active_listeners UInt64,
    listened_ms      UInt64,
    computed_at      DateTime64(3, 'UTC')
)
ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYear(day)
ORDER BY day
TTL day + INTERVAL 5 YEAR DELETE
SETTINGS ttl_only_drop_parts = 1;

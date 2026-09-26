// Package analytics holds the ClickHouse schema of playback analytics: table
// names and versioned migrations — the shared contract of the Analytics
// Worker (writer) and the services that read aggregates (recommendations,
// admin). Apply it with libs/platform/clickhouse Migrate.
package analytics

import "embed"

// Tables.
const (
	TablePlaybackEvents   = "playback_events"
	TableDailyTrackStats  = "daily_track_stats"
	TableDailyArtistStats = "daily_artist_stats"
	TableDailyTotals      = "daily_totals"
)

// Retention of raw events and aggregates, as set by the table TTLs.
const (
	RawRetentionDays        = 180
	AggregateRetentionYears = 5
)

// Migrations is the schema, NNNNN_name.sql files in MigrationsDir.
//
//go:embed migrations/*.sql
var Migrations embed.FS

// MigrationsDir is the directory of Migrations.
const MigrationsDir = "migrations"

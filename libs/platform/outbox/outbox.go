// Package outbox implements the transactional outbox (ADR: at-least-once
// events end to end).
//
// A service writes its Kafka messages into an outbox table in the same
// transaction as the state change (Write), so a message exists if and only
// if the change committed. A Relay publishes the rows in insertion order and
// deletes them once Kafka acknowledged. A crash between publish and delete
// republishes: delivery is at least once and consumers are idempotent.
//
// Each service owns its table in its own schema:
//
//	CREATE TABLE <schema>.outbox (
//	    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
//	    topic      text        NOT NULL,
//	    key        bytea,
//	    value      bytea       NOT NULL,
//	    headers    jsonb       NOT NULL DEFAULT '{}',
//	    created_at timestamptz NOT NULL DEFAULT now()
//	);
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/postgres"
)

// Execer is the part of a pgx connection or transaction Write needs.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// table names are compile-time constants of the services; still, they are
// spliced into SQL, so only schema.table identifiers are accepted.
var tableName = regexp.MustCompile(`^[a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*$`)

func checkTable(table string) error {
	if !tableName.MatchString(table) {
		return fmt.Errorf("outbox: invalid table name %q", table)
	}
	return nil
}

// Write inserts messages into table through q, which must be the
// transaction of the state change. The W3C trace context of ctx is stored
// with each message, so the trace continues when the Relay publishes it.
func Write(ctx context.Context, q Execer, table string, msgs ...kafka.Message) error {
	if err := checkTable(table); err != nil {
		return err
	}
	for _, m := range msgs {
		headers := make(map[string]string, len(m.Headers)+2)
		maps.Copy(headers, m.Headers)
		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))
		raw, err := json.Marshal(headers)
		if err != nil {
			return fmt.Errorf("outbox: headers: %w", err)
		}
		if _, err := q.Exec(ctx, `INSERT INTO `+table+` (topic, key, value, headers) VALUES ($1, $2, $3, $4)`,
			m.Topic, m.Key, m.Value, raw); err != nil {
			return fmt.Errorf("outbox: insert: %w", err)
		}
	}
	return nil
}

// Sink is a drop-in for a Kafka producer that writes to the outbox instead:
// inside postgres.InTx the messages join the caller's transaction. A
// service's event encoder stays unchanged; only its producer is swapped.
type Sink struct {
	pool  *pgxpool.Pool
	table string
}

// NewSink returns a Sink for table (<schema>.outbox).
func NewSink(pool *pgxpool.Pool, table string) (*Sink, error) {
	if err := checkTable(table); err != nil {
		return nil, err
	}
	return &Sink{pool: pool, table: table}, nil
}

// Publish records msgs in the outbox (in the transaction of ctx, if any).
func (s *Sink) Publish(ctx context.Context, msgs ...kafka.Message) error {
	return Write(ctx, postgres.Conn(ctx, s.pool), s.table, msgs...)
}

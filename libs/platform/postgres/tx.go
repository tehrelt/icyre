package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is what both *pgxpool.Pool and pgx.Tx offer. Repositories take
// it from Conn so the same code runs inside or outside a transaction;
// Begin inside a transaction opens a savepoint (pgx.BeginFunc works on it).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type txKey struct{}

// Conn returns the transaction InTx put into ctx, or pool outside one.
func Conn(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

// InTx runs fn in one transaction: everything fn does through Conn(ctx, …)
// commits or rolls back together (a unit of work spanning repositories,
// e.g. an entity and its outbox rows). A nested InTx joins the outer
// transaction.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// Transactor binds InTx to a pool (an application-layer port).
type Transactor struct{ Pool *pgxpool.Pool }

// InTx implements the unit of work of the application layer.
func (t Transactor) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return InTx(ctx, t.Pool, fn)
}

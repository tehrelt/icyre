package postgres

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema is the PostgreSQL schema owned by the Catalog domain.
const Schema = "catalog"

// OutboxTable holds catalog.events messages until the relay sends them.
const OutboxTable = Schema + ".outbox"

// Migrations returns the embedded SQL migrations.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic(err) // embedded path is static
	}
	return sub
}

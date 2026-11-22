// Package postgres implements the Admin Service repositories with pgx.
package postgres

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema is the PostgreSQL schema owned by the Admin domain.
const Schema = "admin"

// Migrations returns the embedded SQL migrations.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic(err) // embedded path is static
	}
	return sub
}

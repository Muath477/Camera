// Package db embeds the SQL migrations.
package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var files embed.FS

// Migrations returns the migration files at the root of the FS.
func Migrations() fs.FS {
	sub, _ := fs.Sub(files, "migrations")
	return sub
}

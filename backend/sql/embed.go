// Package sqlfiles embeds the goose migrations in sql/migrations.
package sqlfiles

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var embedded embed.FS

// Migrations is the migrations directory, rooted at the .sql files.
func Migrations() (fs.FS, error) { return fs.Sub(embedded, "migrations") }

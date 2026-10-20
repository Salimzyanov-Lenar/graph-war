// Package migrations embeds the SQL migration files so the migrate
// command and tests work from a single compiled binary.
package migrations

import "embed"

// FS holds all *.sql files in this directory.
//
//go:embed *.sql
var FS embed.FS

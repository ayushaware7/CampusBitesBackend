// Package migrations embeds the SQL files so cmd/migrate has no runtime file dependency.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

// Package coredb embeds the catalystcal SQL migrations (goose, SQLite dialect).
package coredb

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS

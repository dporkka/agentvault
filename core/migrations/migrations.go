// Package migrations embeds the AgentVault SQL migration files into the binary.
package migrations

import "embed"

// FS contains all .sql migration files in this directory.
//
//go:embed *.sql
var FS embed.FS

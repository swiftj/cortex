// Package migrations provides embedded SQL migrations for Cortex.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

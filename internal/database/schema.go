package database

import _ "embed"

//go:embed sqlc/schema.sql
var Schema string

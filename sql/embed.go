package sqldata

import "embed"

// MigrationsFS contains the sql-migrate files shipped with the project.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS

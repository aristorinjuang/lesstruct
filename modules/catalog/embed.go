package catalog

import "embed"

var (
	//go:embed migrations
	migrationsFS embed.FS
)

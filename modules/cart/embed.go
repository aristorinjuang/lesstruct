package cart

import "embed"

var (
	//go:embed migrations
	migrationsFS embed.FS
)

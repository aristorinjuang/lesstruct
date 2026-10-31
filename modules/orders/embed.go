package orders

import "embed"

var (
	//go:embed migrations
	migrationsFS embed.FS
)

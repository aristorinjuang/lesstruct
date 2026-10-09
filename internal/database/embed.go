package database

import "embed"

var (
	//go:embed migrations/postgresql/*
	postgresqlMigrationsFS embed.FS

	//go:embed migrations/mysql/*
	mysqlMigrationsFS embed.FS
)

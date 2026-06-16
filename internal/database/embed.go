package database

import "embed"

//go:embed migrations/sqlite/*
var sqliteMigrationsFS embed.FS

//go:embed migrations/postgresql/*
var postgresqlMigrationsFS embed.FS

//go:embed migrations/mysql/*
var mysqlMigrationsFS embed.FS

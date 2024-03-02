package main

import (
	_ "github.com/jackc/pgx/v4/stdlib"
	"log/slog"
)

var databaseClient *DatabaseClient

func init() {
	slog.Info("Initializing...")
	databaseClient = NewDatabaseClient()
}

func main() {
	slog.Info("Startup complete!")
}

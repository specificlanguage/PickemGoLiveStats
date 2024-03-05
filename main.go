package main

import (
	_ "github.com/jackc/pgx/v4/stdlib"
	"log/slog"
)

var databaseClient *DatabaseClient

func init() {
	slog.Info("Initializing...")
	databaseClient = NewDatabaseClient()
	slog.Info("Setup complete!")
}

func main() {

	err := handleGameStats(745117, databaseClient)
	if err != nil {
		slog.Error(err.Error())
	}

	finErr := handleGameStats(716643, databaseClient)
	if finErr != nil {
		slog.Error(finErr.Error())
	}

	inProgErr := handleGameStats(748112, databaseClient)
	if inProgErr != nil {
		slog.Error(inProgErr.Error())
	}

}

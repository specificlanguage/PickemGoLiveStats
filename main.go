package main

import (
	"encoding/json"
	_ "github.com/jackc/pgx/v4/stdlib"
	"log/slog"
	"os"
)

var databaseClient *DatabaseClient

func init() {
	databaseClient = NewDatabaseClient()
}

func main() {
	slog.Info("Startup complete!")
	gameData, httpErr := getGameData(716643)
	if httpErr != nil {
		slog.Error(httpErr.Error())
		os.Exit(1)
	}

	gameStats, gameStatsErr := getGameStats(gameData)
	if gameStatsErr != nil {
		os.Exit(1)
	}

	gameInfo, gameInfoErr := unwrap(gameStats, "game")
	if gameInfoErr != nil {
		os.Exit(1)
	}

	readableData, marshalErr := json.Marshal(gameInfo)
	if marshalErr != nil {
		slog.Error(marshalErr.Error())
		os.Exit(1)
	}
	slog.Info(string(readableData))

}

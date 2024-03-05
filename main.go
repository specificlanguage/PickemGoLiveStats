package main

import (
	"context"
	"github.com/jackc/pgtype"
	_ "github.com/jackc/pgx/v4/stdlib"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"
)

var databaseClient *DatabaseClient

type Game struct {
	ID          int         `sql:"id"`
	HomeTeam_ID int         `sql:"home_team_id"`
	AwayTeam_ID int         `sql:"away_team_id"`
	Date        pgtype.Date `sql:"date"`
}

func init() {
	slog.Info("Initializing...")
	databaseClient = NewDatabaseClient()
	slog.Info("Setup complete!")
}

func StatsJob(GameID int, group *sync.WaitGroup) {
	defer group.Done()
	for {
		slog.Info("Running stats job for game " + strconv.Itoa(GameID) + "...")
		finished, err := handleGameStats(GameID, databaseClient)
		if err != nil {
			slog.Error("Error handling game stats for game " + strconv.Itoa(GameID) + "...")
			slog.Error(err.Error())
			break
		}
		if finished {
			slog.Info("Stats job for game " + strconv.Itoa(GameID) + " complete (finished), exiting...")
			break
		}
		slog.Info("Stats job for game " + strconv.Itoa(GameID) + " complete (scheduled or in progress), waiting 20 seconds for next query...")
		time.Sleep(20 * time.Second)
	}
}

func main() {

	// Get information about the games for today
	databaseClient.dbMut.Lock()
	rows, err := databaseClient.db.Query(
		context.Background(),
		`SELECT id, "homeTeam_id", "awayTeam_id", date FROM games WHERE date = $1`,
		time.Now().Format("2006-01-02"))
	if err != nil {
		slog.Error("Could not obtain games for today...")
		slog.Error(err.Error())
		os.Exit(1)
	}

	var games []Game = make([]Game, 0)

	for rows.Next() {
		var game Game
		err := rows.Scan(&game.ID, &game.HomeTeam_ID, &game.AwayTeam_ID, &game.Date)
		if err != nil {
			slog.Error("Could not scan game...")
			slog.Error(err.Error())
			os.Exit(1)
		}
		games = append(games, game)
	}
	rows.Close()
	databaseClient.dbMut.Unlock()

	universalWait := &sync.WaitGroup{}
	for _, game := range games {
		universalWait.Add(1)
		go StatsJob(game.ID, universalWait)
	}
	universalWait.Wait()
	slog.Info("All games for today have been processed, exiting...")

}

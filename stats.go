package main

import (
	"context"
	"log/slog"
	"strconv"
)

// This does the main work of the program, but segmented to each game.
const (
	Scheduled  = "SCHEDULED"
	InProgress = "IN_PROGRESS"
	Completed  = "COMPLETED"
	Unknown    = "UNKNOWN"
)

type ScheduledGameStats struct {
	Status       string `redis:"status"`
	GameID       int    `redis:"gameID"`
	StartTimeUTC string `redis:"startTimeUTC"`
}

type InProgressGameStats struct {
	Status         string
	GameID         int
	HomeScore      int
	AwayScore      int
	CurrentInning  int
	IsTopInning    bool
	CurrentPitcher string
	AtBat          string
	Outs           int
	OnBase         []bool // [first, second, third]
}

type CompletedGameStats struct {
	Status      string
	GameID      int
	HomeScore   int
	AwayScore   int
	WinningTeam int
}

type UnknownGameStats struct {
	status string
	gameID int
}

func handleGameStats(gameID int, dbClient *DatabaseClient) error {
	gameResp, err := getGameData(gameID)
	if err != nil {
		return err
	}

	gameStats, err := getGameStats(gameResp)
	if err != nil {
		return err
	}

	//liveStats, err := getLiveStats(gameResp)
	//if err != nil {
	//	return err
	//}

	gameType, err := getGameType(gameStats)
	switch gameType {
	case Scheduled:
		return handleScheduledGame(gameStats, dbClient)
	}

	return nil
}

func getGameType(gameStats map[string]interface{}) (string, error) {

	gameStatus, gameStatusErr := unwrap(gameStats, "status")
	if gameStatusErr != nil {
		slog.Error(gameStatusErr.Error())
		return "", gameStatusErr
	}

	code := gameStatus["codedGameState"].(string)

	switch code {
	case "S":
		return Scheduled, nil
	case "F":
		return Completed, nil
	case "I":
		return InProgress, nil
	default:
		return Unknown, nil
	}
}

func handleScheduledGame(gameStats map[string]interface{}, client *DatabaseClient) error {

	datetime, datetimeErr := unwrap(gameStats, "datetime")
	if datetimeErr != nil {
		return datetimeErr
	}

	gameID, gameIDErr := unwrap(gameStats, "game")
	if gameIDErr != nil {
		return gameIDErr
	}

	scheduledGameStats := ScheduledGameStats{
		Status:       Scheduled,
		GameID:       int(gameID["pk"].(float64)),
		StartTimeUTC: datetime["dateTime"].(string),
	}

	// Write to redis
	client.redisMut.Lock()
	defer client.redisMut.Unlock()
	rdErr := client.redisClient.HSet(
		context.Background(),
		"game:"+strconv.Itoa(scheduledGameStats.GameID),
		scheduledGameStats,
	).Err()
	if rdErr != nil {
		return rdErr
	}

	slog.Info("Wrote to database")

	return nil
}

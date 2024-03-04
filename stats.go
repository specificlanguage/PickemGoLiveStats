package main

import (
	"context"
	"github.com/redis/go-redis/v9"
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

	// Check if game is already in database, so we don't reassign anything.
	client.redisMut.Lock()

	client.redisMut.Unlock()

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

	_, getRedisErr := client.redisClient.HGet(
		context.Background(),
		"game:"+strconv.Itoa(scheduledGameStats.GameID),
		"status").Result()

	if getRedisErr == redis.Nil { // Could not be found, try writing
		rdErr := client.redisClient.HSet(
			context.Background(),
			"game:"+strconv.Itoa(scheduledGameStats.GameID),
			scheduledGameStats,
		).Err()
		if rdErr != nil {
			return rdErr
		}
		slog.Info("Game " + strconv.Itoa(scheduledGameStats.GameID) + " written to database")
		return nil
	} else if getRedisErr != nil { // Error, return
		return getRedisErr
	} else { // Found, do nothing
		slog.Info("Game " + strconv.Itoa(scheduledGameStats.GameID) + " already in database")
		return nil
	}
}

func handleInProgressGame(gameStats map[string]interface{}, client *DatabaseClient) error {
	return nil
}

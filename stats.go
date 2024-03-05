package main

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"strconv"
	"time"
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
	Status    string `redis:"status"`
	GameID    int    `redis:"gameID"`
	HomeScore int    `redis:"homeScore"`
	AwayScore int    `redis:"awayScore"`
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

	liveStats, err := getLiveStats(gameResp)
	if err != nil {
		return err
	}

	gameType, err := getGameType(gameStats)
	switch gameType {

	case Scheduled:
		return handleScheduledGame(gameStats, dbClient)
	case Completed:
		return handleFinishedGame(
			gameStats, liveStats, dbClient)
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

func handleFinishedGame(gameStats map[string]interface{}, liveData map[string]interface{}, client *DatabaseClient) error {
	// Get Game ID
	gameID, gameIDErr := unwrap(gameStats, "game")
	if gameIDErr != nil {
		return gameIDErr
	}

	// Get final score -- via linescore
	homeScore, awayScore, scoreErr := getScores(liveData)
	if scoreErr != nil {
		return scoreErr
	}

	completedGameStats := CompletedGameStats{
		Status:    Completed,
		GameID:    int(gameID["pk"].(float64)),
		HomeScore: homeScore,
		AwayScore: awayScore,
	}

	// TODO: Handle errors gracefully from these goroutines
	rdErr := setFinalScoreRedis(completedGameStats, client)
	if rdErr != nil {
		return rdErr
	}
	dbErr := setFinalScoreDB(completedGameStats, client)
	if dbErr != nil {
		return dbErr
	}

	return nil
}

func handleInProgressGame(gameStats map[string]interface{}, client *DatabaseClient) error {
	return nil
}

func getScores(liveData map[string]interface{}) (int, int, error) {
	lineScore, lineScoreErr := unwrap(liveData, "linescore")
	if lineScoreErr != nil {
		return -1, -1, lineScoreErr
	}
	teams, teamsErr := unwrap(lineScore, "teams")
	if teamsErr != nil {
		return -1, -1, teamsErr
	}
	home, homeErr := unwrap(teams, "home")
	if homeErr != nil {
		return -1, -1, homeErr
	}
	away, awayErr := unwrap(teams, "away")
	if awayErr != nil {
		return -1, -1, awayErr
	}
	return int(home["runs"].(float64)), int(away["runs"].(float64)), nil
}

func setFinalScoreRedis(stats CompletedGameStats, client *DatabaseClient) error {
	slog.Info("Setting final score in Redis for game " + strconv.Itoa(stats.GameID))
	client.redisMut.Lock()
	defer client.redisMut.Unlock()

	item, merr := json.Marshal(stats)
	if merr != nil {
		return merr
	}
	slog.Info(string(item))

	hsetErr := client.redisClient.HSet(
		context.Background(),
		"game:"+strconv.Itoa(stats.GameID),
		stats).Err()
	if hsetErr != nil {
		return hsetErr
	}

	oneDayDuration := time.Hour * 24
	expireErr := client.redisClient.Expire(
		context.Background(),
		"game:"+strconv.Itoa(stats.GameID),
		oneDayDuration).Err()
	if expireErr != nil {
		return expireErr
	}

	return nil
}

func setFinalScoreDB(stats CompletedGameStats, client *DatabaseClient) error {
	client.dbMut.Lock()
	defer client.dbMut.Unlock()
	_, err := client.db.Query(context.Background(), `
			UPDATE games SET finished=$1, home_score=$2, away_score=$3,
			winner = (
			    CASE WHEN home_score > away_score THEN "homeTeam_id"
			         WHEN home_score < away_score THEN "awayTeam_id"
			         ELSE NULL
			    END
			)    
 			WHERE id = $4`,
		true, stats.HomeScore, stats.AwayScore, stats.GameID)
	return err
}

package main

import (
	"context"
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
	Status         string `redis:"status"`
	GameID         int    `redis:"gameID"`
	HomeScore      int    `redis:"homeScore"`
	AwayScore      int    `redis:"awayScore"`
	CurrentInning  int    `redis:"currentInning"`
	IsTopInning    bool   `redis:"isTopInning"`
	CurrentPitcher string `redis:"currentPitcher"`
	AtBat          string `redis:"atBat"`
	Outs           int    `redis:"outs"`
	OnFirst        bool   `redis:"onFirst"`
	OnSecond       bool   `redis:"onSecond"`
	OnThird        bool   `redis:"onThird"`
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

// Returns true or false if the game that was just handled was finished.
func handleGameStats(gameID int, dbClient *DatabaseClient) (bool, error) {
	gameResp, err := getGameData(gameID)
	if err != nil {
		return false, err
	}

	gameStats, err := getGameStats(gameResp)
	if err != nil {
		return false, err
	}

	liveStats, err := getLiveStats(gameResp)
	if err != nil {
		return false, err
	}

	gameType, err := getGameType(gameStats)
	switch gameType {

	case Scheduled:
		err := handleScheduledGame(gameStats, dbClient)
		return false, err
	case Completed:
		err := handleFinishedGame(gameStats, liveStats, dbClient)
		return true, err
	case InProgress:
		err := handleInProgressGame(gameStats, liveStats, dbClient)
		return false, err
	}

	return false, nil
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
	case "O":
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

func handleInProgressGame(gameStats map[string]interface{}, liveStats map[string]interface{}, client *DatabaseClient) error {
	// Get Game ID
	gameID, gameIDErr := unwrap(gameStats, "game")
	if gameIDErr != nil {
		return gameIDErr
	}

	// Get final score -- via linescore
	homeScore, awayScore, scoreErr := getScores(liveStats)
	if scoreErr != nil {
		return scoreErr
	}

	// Get inning information
	currentInning, isTopInning, outs, inningErr := getInnningInfo(liveStats)
	if inningErr != nil {
		return inningErr
	}

	batterName, pitcherName, onBase, atBatErr := getAtBatInfo(gameStats, liveStats, !isTopInning)
	if atBatErr != nil {
		return atBatErr
	}

	inProgressGameStats := InProgressGameStats{
		Status:         InProgress,
		GameID:         int(gameID["pk"].(float64)),
		HomeScore:      homeScore,
		AwayScore:      awayScore,
		CurrentInning:  currentInning,
		IsTopInning:    isTopInning,
		Outs:           outs,
		AtBat:          batterName,
		CurrentPitcher: pitcherName,
		OnFirst:        onBase[0],
		OnSecond:       onBase[1],
		OnThird:        onBase[2],
	}

	// Write to redis

	client.redisMut.Lock()
	defer client.redisMut.Unlock()
	hsetErr := client.redisClient.HSet(
		context.Background(),
		"game:"+strconv.Itoa(inProgressGameStats.GameID),
		inProgressGameStats).Err()
	if hsetErr != nil {
		return hsetErr
	}

	quickDuration := time.Minute * 10
	expireErr := client.redisClient.Expire(
		context.Background(),
		"game:"+strconv.Itoa(inProgressGameStats.GameID),
		quickDuration).Err()
	if expireErr != nil {
		return expireErr
	}

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

// Get inning information from the live data, returns the current inning and whether it is the top of the inning.
func getInnningInfo(liveData map[string]interface{}) (int, bool, int, error) {
	linescore, linescoreErr := unwrap(liveData, "linescore")
	if linescoreErr != nil {
		return -1, false, -1, linescoreErr
	}
	return int(linescore["currentInning"].(float64)),
		linescore["isTopInning"].(bool),
		int(linescore["outs"].(float64)),
		nil
}

// Returns batter name, pitcher name, a list of length 3 for onFirst, onSecond, onThird, error
func getAtBatInfo(gameData map[string]interface{}, liveData map[string]interface{}, isBottomInning bool) (string, string, []bool, error) {
	keys := [3]string{"plays", "currentPlay", "matchup"}
	matchupData := liveData
	for k := range keys {
		unwrapped, unwrappedErr := unwrap(matchupData, keys[k])
		if unwrappedErr != nil {
			return "", "", []bool{false, false, false}, unwrappedErr
		}
		matchupData = unwrapped
	}

	// Batter info (please note that we will eventually have to swap to statcast data to get batting average)
	batterInfo, batterErr := unwrap(matchupData, "batter")
	if batterErr != nil {
		return "", "", []bool{false, false, false}, batterErr
	}
	batterID := int(batterInfo["id"].(float64))
	batterName, batterErr := getPlayerName(gameData, batterID)
	if batterErr != nil {
		return "", "", []bool{false, false, false}, batterErr
	}

	// Pitcher info (please note that we will eventually have to swap to statcast data to get ERA, pitches thrown)
	pitcherInfo, pitcherErr := unwrap(matchupData, "pitcher")
	if pitcherErr != nil {
		return "", "", []bool{false, false, false}, pitcherErr
	}
	pitcherID := int(pitcherInfo["id"].(float64))
	pitcherName, pitcherErr := getPlayerName(gameData, pitcherID)
	if pitcherErr != nil {
		return "", "", []bool{false, false, false}, pitcherErr
	}

	// On base info
	onBase := make([]bool, 3)
	_, firstErr := unwrap(matchupData, "postOnFirst")
	if firstErr == nil { // In a surprising twist we're actually going to ignore errors, because we don't care if they're not on base
		onBase[0] = true
	}

	_, secondErr := unwrap(matchupData, "postOnSecond")
	if secondErr == nil {
		onBase[1] = true
	}

	_, thirdErr := unwrap(matchupData, "postOnThird")
	if thirdErr == nil {
		onBase[2] = true
	}

	return batterName, pitcherName, onBase, nil

}

// Returns a player's box score name
func getPlayerName(gameData map[string]interface{}, playerID int) (string, error) {
	players, playersErr := unwrap(gameData, "players")
	if playersErr != nil {
		return "", playersErr
	}

	//bytes, err := json.Marshal(players)
	//if err != nil {
	//	return "", err
	//}
	//slog.Info(string(bytes))
	//slog.Info("ID" + strconv.Itoa(playerID))

	player, playerErr := unwrap(players, "ID"+strconv.Itoa(playerID))
	if playerErr != nil {
		return "", playerErr
	}

	return player["boxscoreName"].(string), nil
}

func setFinalScoreRedis(stats CompletedGameStats, client *DatabaseClient) error {
	slog.Info("Setting final score in Redis for game " + strconv.Itoa(stats.GameID))
	client.redisMut.Lock()
	defer client.redisMut.Unlock()

	//item, merr := json.Marshal(stats)
	//if merr != nil {
	//	return merr
	//}
	//slog.Info(string(item))

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
	cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	tx, err := client.db.Begin(cctx)

	defer cancel()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback(context.Background())
		} else {
			tx.Commit(context.Background())
		}
	}()

	_, err = client.db.Exec(cctx, `
			UPDATE games SET finished=$1, home_score=$2, away_score=$3,
			winner = (
			    CASE WHEN home_score > away_score THEN "homeTeam_id"
			         WHEN home_score < away_score THEN "awayTeam_id"
			         ELSE NULL
			    END
			)
 			WHERE id = $4`,
		true, stats.HomeScore, stats.AwayScore, stats.GameID)

	if err != nil {
		return err
	}

	_, err = client.db.Exec(cctx, `
		UPDATE picks set correct=
		 (CASE WHEN "pickedHome" = 
			(SELECT winner = "homeTeam_id" from games where id = $1 and winner IS NOT NULL) 
		  THEN true 
		 ELSE false END) WHERE game_id = $1`, stats.GameID)

	if err != nil {
		return err
	}

	err = tx.Commit(context.Background())
	return err
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

func getGameData(gameID int) (map[string]interface{}, error) {
	resp, respErr := http.Get(fmt.Sprintf("https://statsapi.mlb.com/api/v1.1/game/%d/feed/live", gameID))
	if respErr != nil {
		return nil, respErr
	}
	defer resp.Body.Close()

	body, ioErr := io.ReadAll(resp.Body)
	if ioErr != nil {
		return nil, ioErr
	}

	var mlbDataJSON map[string]interface{}
	if unmarshalErr := json.Unmarshal(body, &mlbDataJSON); unmarshalErr != nil {
		return nil, unmarshalErr
	}

	return mlbDataJSON, nil
}

func getGameStats(gameData map[string]interface{}) (map[string]interface{}, error) {
	return unwrap(gameData, "gameData")
}

func getLiveStats(gameData map[string]interface{}) (map[string]interface{}, error) {
	return unwrap(gameData, "liveData")
}

func unwrap(jsonData map[string]interface{}, key string) (map[string]interface{}, error) {
	nestedData, ok := jsonData[key].(map[string]interface{})
	if !ok {
		return nil, errors.New(fmt.Sprintf("Key %s not found in JSON data", key))
	}
	return nestedData, nil
}

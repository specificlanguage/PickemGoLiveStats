package main

import (
	"context"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"os"
)

var db *pgxpool.Pool

var redisClient *redis.Client

func init() {

	slog.Info("Initializing...")
	dotEnvErr := godotenv.Load()
	if dotEnvErr != nil {
		slog.Error("Error loading .env file, exiting")
		os.Exit(1)
	}

	// Connect to database
	connStr := os.Getenv("DATABASE_URL")
	slog.Debug(connStr)

	sqlDB, dbErr := pgxpool.Connect(context.Background(), connStr)
	if dbErr != nil {
		slog.Error(dbErr.Error())
		slog.Error("Error connecting to database, exiting")
		os.Exit(1)
	}
	db = sqlDB

	// Connect to Redis
	redisAddr := os.Getenv("REDIS_URL")
	opt, redisErr := redis.ParseURL(redisAddr)
	if redisErr != nil {
		slog.Error("Error connecting to Redis, exiting")
		os.Exit(1)
	}
	redisClient = redis.NewClient(opt)
}

func main() {
	slog.Info("Startup complete!")
	slog.Info(redisClient.String())
	var result string
	err := db.QueryRow(context.Background(), "SELECT 'hello world!'").Scan(&result)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	slog.Info(result)
}

package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"os"
	"sync"
)

type DatabaseClient struct {
	db          *pgxpool.Pool
	redisClient *redis.Client
	dbMut       *sync.Mutex
	redisMut    *sync.Mutex
}

func NewDatabaseClient() *DatabaseClient {
	godotenv.Load()
	//if dotEnvErr != nil {
	//	slog.Error("Error loading .env file, exiting")
	//	os.Exit(1)
	//}

	// Connect to database
	connStr := os.Getenv("DATABASE_URL")
	slog.Debug(connStr)

	sqlDB, dbErr := pgxpool.New(context.Background(), connStr)
	if dbErr != nil {
		slog.Error(dbErr.Error())
		slog.Error("Error connecting to database, exiting")
		os.Exit(1)
	}

	// Connect to Redis
	redisAddr := os.Getenv("REDIS_URL")
	opt, redisErr := redis.ParseURL(redisAddr)
	if redisErr != nil {
		slog.Error("Error connecting to Redis, exiting")
		os.Exit(1)
	}
	redisClient := redis.NewClient(opt)

	return &DatabaseClient{
		db:          sqlDB,
		redisClient: redisClient,
		dbMut:       &sync.Mutex{},
		redisMut:    &sync.Mutex{},
	}

}

package connection

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
	"sync"
	"user-management/model"
)

var (
	DB          *gorm.DB
	once        sync.Once
	redisClient *redis.Client
	ctx         = context.TODO()
)

func InitDB() (*gorm.DB, error) {
	var err error
	once.Do(func() {
		host := os.Getenv("DB_HOST")
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")
		dbname := os.Getenv("DB_NAME")

		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=5432 sslmode=require", host, user, password, dbname)

		var err error
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}

		log.Println("Successfully connected to the database")

		err = DB.AutoMigrate(&model.User{}, &model.Contact{})
		if err != nil {
			log.Fatalf("failed to migrate database: %v", err)
		}
		log.Println("Migrated")
	})
	return DB, err
}

// InitRedis Initialize Redis client to AWS ElastiCache
func InitRedis() (*redis.Client, error) {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		log.Fatalf("REDIS_HOST environment variable is not set")
	}

	log.Println("Connecting to Redis at:", redisHost)

	redisClient = redis.NewClient(&redis.Options{
		Addr:      redisHost,
		Password:  "",
		TLSConfig: nil,
	})

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("failed to connect to Redis: %v", err)
	}

	log.Println("Successfully connected to Redis")
	return redisClient, err
}

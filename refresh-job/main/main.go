package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"user-management/connection"
	"user-management/services"
)

func handler() error {
	dsn := os.Getenv("DB_DSN")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	redis, err := connection.InitRedis()
	if err != nil {
		return err
	}

	us := services.UserService{DB: db,
		RedisClient: redis}
	return us.RefreshAllUserCache()
}

func main() {
	lambda.Start(handler)
}

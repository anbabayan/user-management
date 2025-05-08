package db

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
	"sync"
	"user-management/model"
)

var (
	DB   *gorm.DB
	once sync.Once
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

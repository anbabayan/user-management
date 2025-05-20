package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
	"log"
	"os"
	"strings"
	"time"
	"user-management/model"
)

var redisClient *redis.Client
var ctx = context.TODO()

type UserService struct {
	DB *gorm.DB
}

type S3Client struct {
	Client *s3.Client
}

// InitRedis Initialize Redis client to AWS ElastiCache
func InitRedis() error {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		return fmt.Errorf("REDIS_HOST environment variable is not set")
	}

	fmt.Println("Connecting to Redis at:", redisHost)

	redisClient = redis.NewClient(&redis.Options{
		Addr:      redisHost,
		Password:  "",
		TLSConfig: nil,
	})

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	fmt.Println("Successfully connected to Redis")
	return nil
}

func (s *UserService) CreateUser(user *model.User) error {
	return s.DB.Debug().Create(user).Error
}

func (s *UserService) GetUserByID(id string) (*model.User, error) {
	cacheKey := "user:" + id
	log.Println("Redis Client", redisClient)
	userJson, err := redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		log.Println("User found in Redis cache")
		var user model.User
		err := json.Unmarshal([]byte(userJson), &user)
		if err != nil {
			return nil, errors.New("failed to unmarshal user data from cache")
		}
		return &user, nil
	}

	log.Println("User found in DB")
	var user model.User
	err = s.DB.Debug().Preload("Contacts").First(&user, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	// Store the fetched user in Redis cache for 30 minutes
	userData, err := json.Marshal(user)
	if err != nil {
		return nil, errors.New("failed to marshal user data to cache")
	}
	err = redisClient.Set(ctx, cacheKey, userData, 30*time.Minute).Err()
	if err != nil {
		log.Printf("Error setting user data to Redis: %v", err)
	}

	return &user, nil
}

func (s *UserService) UpdateUser(user *model.User) error {
	return s.DB.Debug().Transaction(func(tx *gorm.DB) error {
		log.Printf("User update id  %v\n", user.ID)
		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).
			Updates(map[string]interface{}{
				"avatar":   user.Avatar,
				"username": user.Username,
				"name":     user.Name,
				"password": user.Password,
				"status":   user.Status,
			}).Error; err != nil {
			return err
		}

		if err := tx.Where("user_id = ?", user.ID).Delete(&model.Contact{}).Error; err != nil {
			return err
		}

		for i := range user.Contacts {
			user.Contacts[i].UserID = user.ID
		}
		if len(user.Contacts) > 0 {
			if err := tx.Create(&user.Contacts).Error; err != nil {
				return err
			}
		}

		cacheKey := "user:" + user.ID
		err := redisClient.Del(ctx, cacheKey).Err()
		if err != nil {
			log.Printf("Error deleting user cache: %v", err)
		}

		return nil
	})
}

func (s *UserService) UpdateUserAvatar(userID string, avatarUrl string) error {
	return s.DB.Debug().Model(&model.User{}).Where("id = ?", userID).Update("avatar", avatarUrl).Error
}

func (s *UserService) DeleteUser(id string) error {
	return s.DB.Debug().Delete(&model.User{}, "id = ?", id).Error
}

func (s *UserService) ListUsers(filter map[string]string) ([]model.User, error) {
	var users []model.User
	query := s.DB.Debug().Preload("Contacts")
	if status, ok := filter["status"]; ok {
		query = query.Where("status = ?", status)
	}
	if search, ok := filter["search"]; ok {
		query = query.Where("username ILIKE ? OR name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	return users, query.Find(&users).Error
}

func NewS3Client() (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Printf("unable to load SDK config: %v", err)
		return nil, errors.New("unable to load SDK config")
	}
	return &S3Client{Client: s3.NewFromConfig(cfg)}, nil
}

func (s *UserService) PutObject(key, data string) error {
	bucketName := os.Getenv("BUCKET_NAME")

	s3Client, err := NewS3Client()
	if err != nil {
		log.Fatalf("Error creating S3 client: %v", err)
	}

	_, err = s3Client.Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      &bucketName,
		Key:         &key,
		ContentType: aws.String("image/png"),
		Body:        strings.NewReader(data),
	})
	if err != nil {
		log.Printf("failed to put object: %v", err)
		return errors.New("failed to put object")
	}
	return nil
}

// RefreshAllUserCache refreshes the cache for all users
func (s *UserService) RefreshAllUserCache() error {
	log.Println("Starting cache refresh for all users...")

	var users []model.User
	err := s.DB.Preload("Contacts").Find(&users).Error
	if err != nil {
		log.Printf("Error fetching users from DB: %v", err)
		return fmt.Errorf("failed to fetch users: %w", err)
	}

	successCount := 0
	failCount := 0

	for _, user := range users {
		cacheKey := "user:" + user.ID
		userJson, err := json.Marshal(user)
		if err != nil {
			log.Printf("Failed to marshal user %v: %v", user.ID, err)
			failCount++
			continue
		}

		err = redisClient.Set(ctx, cacheKey, userJson, 30*time.Minute).Err()
		if err != nil {
			log.Printf("Failed to cache user %v in Redis: %v", user.ID, err)
			failCount++
			continue
		}

		successCount++
	}

	log.Printf("Cache refresh complete. Success: %d, Failures: %d", successCount, failCount)
	return nil
}

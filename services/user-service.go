package services

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gorm.io/gorm"
	"log"
	"os"
	"strings"
	"user-management/model"
)

type UserService struct {
	DB *gorm.DB
}

type S3Client struct {
	Client *s3.Client
}

func (s *UserService) CreateUser(user *model.User) error {
	return s.DB.Create(user).Error
}

func (s *UserService) GetUserByID(id string) (*model.User, error) {
	var user model.User
	// todo change condition
	//todo check transaction
	// todo check preload
	err := s.DB.Preload("Contacts").First(&user, "id = ?", id).Error
	return &user, err
}

func (s *UserService) UpdateUser(user *model.User) error {
	return s.DB.Debug().Transaction(func(tx *gorm.DB) error {
		// Update user fields
		// todo check printed queries
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

		// Delete existing contacts
		if err := tx.Where("user_id = ?", user.ID).Delete(&model.Contact{}).Error; err != nil {
			return err
		}

		// Insert new contacts
		for i := range user.Contacts {
			user.Contacts[i].UserID = user.ID
		}
		if len(user.Contacts) > 0 {
			if err := tx.Create(&user.Contacts).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *UserService) UpdateUserAvatar(userID string, avatarUrl string) error {
	return s.DB.Model(&model.User{}).Where("id = ?", userID).Update("avatar", avatarUrl).Error
}

func (s *UserService) DeleteUser(id string) error {
	return s.DB.Delete(&model.User{}, "id = ?", id).Error
}

func (s *UserService) ListUsers(filter map[string]string) ([]model.User, error) {
	var users []model.User
	query := s.DB.Preload("Contacts")
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

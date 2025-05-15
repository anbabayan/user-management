package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log"
	"net/http"
	"time"
	"user-management/db"
	"user-management/model"
	"user-management/services"
)

var userService *services.UserService

type S3Client struct {
	Client *s3.Client
}

func init() {
	database, err := db.InitDB()
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to DB: %v", err))
	}
	userService = &services.UserService{DB: database}

	// Initialize Redis client
	err = services.InitRedis()
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to Redis: %v", err))
	}
}

func apiGatewayHandler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	switch req.HTTPMethod {
	case "POST":
		if req.Path == "/upload" {
			return handleUploadAvatar(req)
		}
		return handleCreateUser(req)
	case "GET":
		id := req.PathParameters["id"]
		if id != "" {
			return handleGetUserByID(req)
		}
		return handleListUsers(req)
	case "PUT":
		return handleUpdateUser(req)
	}
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNotFound,
		Body:       "Not Found",
	}, nil
}

func eventBridgeHandler(ctx context.Context, event events.CloudWatchEvent) (string, error) {
	log.Println("EventBridge triggered cache refresh")

	if err := userService.RefreshAllUserCache(); err != nil {
		log.Printf("Error refreshing cache: %v", err)
		return "Failed to refresh cache", err
	}

	return "User cache refresh complete", nil
}

func handler(ctx context.Context, rawEvent json.RawMessage) (interface{}, error) {
	// Try to unmarshal as API Gateway event
	var apiReq events.APIGatewayProxyRequest
	if err := json.Unmarshal(rawEvent, &apiReq); err == nil && apiReq.HTTPMethod != "" {
		return apiGatewayHandler(ctx, apiReq)
	}

	// Try to unmarshal as EventBridge event
	var ebEvent events.CloudWatchEvent
	if err := json.Unmarshal(rawEvent, &ebEvent); err == nil && ebEvent.Source != "" {
		return eventBridgeHandler(ctx, ebEvent)
	}

	log.Println("Unknown event format")
	return nil, fmt.Errorf("unsupported event format")
}

func handleCreateUser(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var user model.User
	if err := json.Unmarshal([]byte(req.Body), &user); err != nil {
		return errorResponse(http.StatusBadRequest, "Invalid request body")
	}

	// Validate required fields
	if user.Username == "" || user.Password == "" || user.Status == "" || len(user.Contacts) == 0 {
		return errorResponse(http.StatusBadRequest, "Missing required fields")
	}

	if err := userService.CreateUser(&user); err != nil {
		return errorResponse(http.StatusInternalServerError, "Failed to create user")
	}

	resBody, _ := json.Marshal(user)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusCreated,
		Body:       string(resBody),
	}, nil
}

func handleGetUserByID(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	id := req.PathParameters["id"]
	if id == "" {
		return errorResponse(http.StatusBadRequest, "Missing user ID")
	}

	user, err := userService.GetUserByID(id)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Failed to retrieve user")
	}
	if user == nil {
		return errorResponse(http.StatusNotFound, "User not found")
	}

	resBody, _ := json.Marshal(user)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(resBody),
	}, nil
}

func handleListUsers(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	filter := map[string]string{}
	if status := req.QueryStringParameters["status"]; status != "" {
		filter["status"] = status
	}
	if search := req.QueryStringParameters["search"]; search != "" {
		filter["search"] = search
	}

	users, err := userService.ListUsers(filter)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Failed to fetch users")
	}

	resBody, _ := json.Marshal(users)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(resBody),
	}, nil
}

func handleUpdateUser(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userID := req.PathParameters["id"]
	if userID == "" {
		return errorResponse(http.StatusBadRequest, "Missing user ID")
	}

	var input model.User
	if err := json.Unmarshal([]byte(req.Body), &input); err != nil {
		return errorResponse(http.StatusBadRequest, "Invalid JSON body")
	}

	input.ID = userID // Enforce ID from path param

	if err := userService.UpdateUser(&input); err != nil {
		log.Printf("Update error: %v", err)
		return errorResponse(http.StatusInternalServerError, "Failed to update user")
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       `{"message": "User updated successfully"}`,
	}, nil
}

// Handle file upload (POST /upload)
func handleUploadAvatar(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Parse the base64 image data from the request body
	var requestBody struct {
		ImageData string `json:"image_data"`
	}

	userID := req.QueryStringParameters["id"]
	if userID == "" {
		return errorResponse(http.StatusBadRequest, "Missing userId")
	}

	if err := json.Unmarshal([]byte(req.Body), &requestBody); err != nil {
		return errorResponse(http.StatusBadRequest, "Invalid request body")
	}

	// Validate image data
	if requestBody.ImageData == "" {
		return errorResponse(http.StatusBadRequest, "No image data provided")
	}

	// Decode the base64 image data
	decodedData, err := base64.StdEncoding.DecodeString(requestBody.ImageData)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Failed to decode image data")
	}

	// Generate a unique file name for the avatar
	fileName := fmt.Sprintf("avatars/%s.png", generateUUID())

	// Upload the avatar image to S3
	err = userService.PutObject(fileName, string(decodedData))
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Failed to upload image to S3")
	}

	// Update user avatar in DB
	err = userService.UpdateUserAvatar(userID, fileName)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Failed to update user")
	}

	// Return the file path (or S3 URL) in the response
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       fmt.Sprintf(`{"file_path": "%s"}`, fileName),
	}, nil
}

// Helper function to generate a unique UUID (can be replaced with a UUID library)
func generateUUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func errorResponse(status int, message string) (events.APIGatewayProxyResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"error": message,
	})
	return events.APIGatewayProxyResponse{
		StatusCode: status,
		Body:       string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}

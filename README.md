# User Management Service (Go + AWS Lambda)

This project is a **Go-based AWS Lambda function** that provides a user management system with a RESTful API exposed via **API Gateway**, secured using **Cognito authentication**. It integrates with **S3** for avatar uploads, **RDS (PostgreSQL)** for persistent storage, **Redis** for caching, and uses a **VPC endpoint** for secure S3 access.

---

## 🚀 Features

### User API Endpoints

| Method | Endpoint       | Description                                  |
|--------|----------------|----------------------------------------------|
| POST   | `/upload/{id}` | Uploads a user avatar to S3 and returns its path |
| POST   | `/user`        | Creates a new user with optional avatar and contacts |
| GET    | `/user`        | Retrieves users with filtering & search support |
| GET    | `/user/{id}`   | Retrieves a single user by ID (cached in Redis) |
| PUT    | `/user/{id}`   | Updates an existing user's data              |

### User Object

```json
{
  "id": "auto-generated",
  "avatar": "optional S3 path",
  "username": "string (required, unique)",
  "name": "string (optional)",
  "password": "string (required)",
  "status": "ACTIVE | BLOCKED",
  "contacts": [
    {
      "id": "auto-generated",
      "contact_type": "PHONE | WORK | WHATSAPP",
      "value": "string"
    }
  ]
}
```

---

## 🛠️ Architecture & Technical Stack

| Component      | Details                                                     |
|----------------|-------------------------------------------------------------|
| **Language**   | Go (Golang)                                                 |
| **API Layer**  | Amazon API Gateway                                          |
| **Lambda**     | Go-based function deployed via GitHub Actions CI/CD         |
| **Database**   | Amazon RDS (PostgreSQL) with GORM ORM                       |
| **Caching**    | Amazon ElastiCache (Redis), with daily cache revalidation via EventBridge |
| **File Storage** | Amazon S3 (accessed via VPC Gateway Endpoint)               |
| **Auth**       | AWS Cognito JWT authentication via API Gateway authorizers  |
| **Monitoring** | AWS CloudWatch Logs                                         |

---

## 🔐 Authentication

All routes are protected by **AWS Cognito** and require valid JWT tokens. Authorization is handled via **API Gateway Authorizers**.

---

## 🧠 Caching Strategy

- **Redis** is used to cache `GET /user/{id}` responses.
- Cache entries are invalidated or refreshed on user updates.
- A scheduled job (via **Amazon EventBridge**) runs every 24 hours to **revalidate cached user data**.

---

## 📦 Project Structure

```
.
├── connection/             # handles connections to RDS and Redis
├── handler/                # HTTP handlers
├── model/                  # GORM models (e.g., User, Contact)
├── refresh-job/            # Periodic job logic
├── services/               # Business logic
├── .github/                # GitHub workflow
├── go.mod                  # Go module definition
├── go.work                 # Go workspace config
└── README.md               # README file
```

---

## 🚧 CI/CD

GitHub Actions pipeline for:

- ✅ Linting and testing
- ✅ Building Lambda ZIP/package
- ✅ Deployment to AWS
- ✅ Lambda alias versioning

---

## 🐞 Error Handling

- JSON error responses with appropriate HTTP status codes
- Structured logs for debugging and CloudWatch insights

---

## 📈 Monitoring

- Logs sent to **CloudWatch**

---

## ✅ Deployment

1. Make sure AWS credentials and required permissions are configured.
2. GitHub Actions deploys automatically on push to `main` (configurable).
3. API is accessible via the following endpoint:

```
https://w2qz2hpo6g.execute-api.eu-central-1.amazonaws.com/test
```

---

## 📋 Requirements

- Go 1.20+
- AWS CLI configured
- GitHub Actions secrets for deployment (if deploying via CI)

---


## 👤 Author

Developed by [@anbabayan](https://github.com/anbabayan)

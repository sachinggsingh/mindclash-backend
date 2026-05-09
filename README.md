# MindClash Backend

The backend server for MindClash, a high-performance quiz application built with Go and MongoDB. It handles user authentication, quiz management, scoring, and real-time leaderboard updates via WebSockets.

## 🚀 Features

- **User Authentication**: Secure signup and login using JWT (Access & Refresh tokens).
- **Quiz Management**: Create and fetch quizzes with multiple-choice questions.
- **Scoring System**: Automated point calculation based on correct answers.
- **Real-time Leaderboard**: Instant updates for all connected clients using WebSockets.
- **User Profiles**: Track scores, streaks, and activity history.
- **CORS Enabled**: Configured for seamless communication with the Next.js frontend.

## 🛠 Tech Stack

- **Language**: [Go (Golang)](https://golang.org/)
- **Framework**: [Gorilla Mux](https://github.com/gorilla/mux)
- **Database**: [MongoDB](https://www.mongodb.com/)
- **Authentication**: [JWT (JSON Web Tokens)](https://github.com/golang-jwt/jwt)
- **Real-time**: [WebSockets](https://github.com/gorilla/websocket)

## 📁 Project Structure

```text
quiz-backend/
├── cmd/
│   └── server/          # Application entry point
├── config/              # Configuration and DB connection
├── internal/
│   ├── api/             # API Router and Server setup
│   │   └── handler/     # HTTP Handlers
│   ├── model/           # BSON and JSON models
│   ├── repo/            # Database repositories (DAO)
│   ├── service/         # Business logic layer
│   └── utils/           # Helper utilities (JWT, etc.)
└── .env                 # Environment variables
```

## 📡 API Endpoints

### Authentication
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| POST | `/users` | Register a new user |
| POST | `/login` | Login and receive JWT tokens |
| POST | `/refresh-token` | Refresh access token |
| GET | `/me` | Get current user profile (Auth required) |

### Quizzes
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| GET | `/quizzes` | Fetch all available quizzes |
| GET | `/quizzes/{id}` | Get specific quiz details |
| POST | `/quizzes/{id}/submit` | Submit answers and get score (Auth required) |

### Real-time
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| WS | `/ws/leaderboard` | WebSocket connection for live leaderboard updates |

## ⚙️ Setup & Installation

1. **Clone the repository**:
   ```bash
   git clone https://github.com/sachinggsingh/quizy-backend.git
   cd quizy-backend
   ```

2. **Configure Environment Variables**:
   Create a `.env` file in the root directory:
   ```env
   PORT=3003
   MONGO_URI=your_mongodb_connection_string
   DB_NAME=quiz_db
   JWT_KEY=your_secret_key
   ```

3. **Install dependencies**:
   ```bash
   go mod download
   ```

4. **Run the server**:
   ```bash
   go run ./cmd/server/main.go
   ```

---
Built with ❤️ by [Sachin Singh](https://github.com/sachinggsingh)

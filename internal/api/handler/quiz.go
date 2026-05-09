package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/sachinggsingh/quiz/internal/model"
	"github.com/sachinggsingh/quiz/internal/service"
	"github.com/sachinggsingh/quiz/internal/telemetry"
	"github.com/sachinggsingh/quiz/internal/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

type QuizHandler struct {
	quizService *service.QuizService
	userService *service.UserService
}

func NewQuizHandler(quizService *service.QuizService, userService *service.UserService) *QuizHandler {
	return &QuizHandler{
		quizService: quizService,
		userService: userService,
	}
}

func (h *QuizHandler) GenerateQuiz(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("GenerateQuiz").Start(r.Context(), "GenerateQuiz")
	defer span.End()
	var req struct {
		Title        string `json:"title"`
		Category     string `json:"category"`
		Difficulty   string `json:"difficulty"`
		Description  string `json:"description"`
		NumQuestions int    `json:"num_questions"`
		Points       int    `json:"points"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to decode quiz", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate basic inputs if necessary
	if req.NumQuestions <= 0 {
		req.NumQuestions = 5 // Default
	}
	if req.Points <= 0 {
		req.Points = 100 // Default
	}

	generatedQuiz, err := h.quizService.GenerateQuiz(
		ctx,
		req.Title,
		req.Category,
		req.Difficulty,
		req.Description,
		req.NumQuestions,
		req.Points,
	)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to generate quiz", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	telemetry.LogWithTrace(ctx).Info("quiz generated successfully", zap.Any("quiz", generatedQuiz))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(generatedQuiz)
}

func (h *QuizHandler) CreateQuiz(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("Create-Quiz").Start(r.Context(), "Create-Quiz")
	defer span.End()
	var quiz model.Quiz
	if err := json.NewDecoder(r.Body).Decode(&quiz); err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to decode quiz", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	created, err := h.quizService.CreateQuiz(ctx, quiz.Title, quiz.Category, quiz.Difficulty, quiz.Questions, quiz.Points)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to create quiz", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	telemetry.LogWithTrace(ctx).Info("quiz created successfully", zap.Any("quiz", created))
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (h *QuizHandler) GetQuizzesGroupedByCategory(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("GetQuizzesGroupedByCategory").Start(r.Context(), "GetQuizzesGroupedByCategory")
	defer span.End()
	// Try to get userID if authenticated
	var userID primitive.ObjectID
	tokenString := utils.GetTokenFromRequest(r)
	if tokenString != "" {
		telemetry.LogWithTrace(ctx).Info("token string", zap.String("token", tokenString))
		token, err := utils.TokenValidator(tokenString)
		if err == nil && token.Valid {
			telemetry.LogWithTrace(ctx).Info("token is valid")
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				telemetry.LogWithTrace(ctx).Info("token claims", zap.Any("claims", claims))
				if userIDHex, ok := claims["user_id"].(string); ok {
					telemetry.LogWithTrace(ctx).Info("user id", zap.String("user_id", userIDHex))
					userID, _ = primitive.ObjectIDFromHex(userIDHex)
				}
			}
		}
	}

	grouped, err := h.quizService.GetQuizzesGroupedByCategory(ctx, userID)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to get quizzes grouped by category", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	telemetry.LogWithTrace(ctx).Info("quizzes grouped by category", zap.Any("grouped", grouped))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(grouped)
}

func (h *QuizHandler) GetQuizzes(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("GetQuizzes").Start(r.Context(), "GetQuizzes")
	defer span.End()
	// Try to get userID if authenticated (optional auth)
	var userID primitive.ObjectID
	tokenString := utils.GetTokenFromRequest(r)
	if tokenString != "" {
		telemetry.LogWithTrace(ctx).Info("token string", zap.String("token", tokenString))
		token, err := utils.TokenValidator(tokenString)
		if err == nil && token.Valid {
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				if userIDHex, ok := claims["user_id"].(string); ok {
					telemetry.LogWithTrace(ctx).Info("user id", zap.String("user_id", userIDHex))
					userID, _ = primitive.ObjectIDFromHex(userIDHex)
				}
			}
		}
	}

	// Pass userID to the service to decorate quizzes with Attempted status
	quizzes, err := h.quizService.GetQuizzes(ctx, userID)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to get quizzes", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	telemetry.LogWithTrace(ctx).Info("quizzes", zap.Any("quizzes", quizzes))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(quizzes)
}

func (h *QuizHandler) GetQuiz(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("GetQuiz").Start(r.Context(), "GetQuiz")
	defer span.End()
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("invalid quiz id", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "invalid quiz id", http.StatusBadRequest)
		return
	}

	quiz, err := h.quizService.GetQuizByID(ctx, id)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to get quiz", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	telemetry.LogWithTrace(ctx).Info("quiz", zap.Any("quiz", quiz))

	// Debug: Log quiz data before sending
	fmt.Printf("Quiz fetched - ID: %s, Title: %s, Questions count: %d\n", quiz.ID.Hex(), quiz.Title, len(quiz.Questions))
	if len(quiz.Questions) > 0 {
		telemetry.LogWithTrace(ctx).Info("First question", zap.String("question", quiz.Questions[0].Text))
		fmt.Printf("First question - ID: %s, Text: %s, Options count: %d\n",
			quiz.Questions[0].ID.Hex(), quiz.Questions[0].Text, len(quiz.Questions[0].Options))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(quiz)
}

func (h *QuizHandler) SubmitQuizResult(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("SubmitQuizResult").Start(r.Context(), "SubmitQuizResult")
	defer span.End()
	// User ID is already set in context by Authenticate middleware
	userIDHex := utils.GetUserId(r.Context())
	if userIDHex == "" {
		telemetry.LogWithTrace(ctx).Error("user not authenticated")
		span.SetStatus(codes.Error, "user not authenticated")
		http.Error(w, "user not authenticated", http.StatusUnauthorized)
		return
	}

	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("invalid user id", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	var req struct {
		QuizID string `json:"quiz_id"`
		Score  int    `json:"score"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to decode quiz", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	quizID, err := primitive.ObjectIDFromHex(req.QuizID)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("invalid quiz id", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "invalid quiz id", http.StatusBadRequest)
		return
	}

	if err := h.userService.SubmitQuizResult(ctx, userID, quizID, req.Score); err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to submit quiz result", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *QuizHandler) SubmitQuiz(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("SubmitQuiz").Start(r.Context(), "SubmitQuiz")
	defer span.End()
	vars := mux.Vars(r)
	quizIDStr := vars["id"]
	quizID, err := primitive.ObjectIDFromHex(quizIDStr)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("invalid quiz id", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "invalid quiz id", http.StatusBadRequest)
		return
	}
	span.SetAttributes(attribute.String("quiz.id", quizIDStr))

	// User ID is already set in context by Authenticate middleware
	userIDHex := utils.GetUserId(r.Context())
	if userIDHex == "" {
		telemetry.LogWithTrace(ctx).Error("user not authenticated")
		span.SetStatus(codes.Error, "user not authenticated")
		http.Error(w, "user not authenticated", http.StatusUnauthorized)
		return
	}

	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("invalid user id", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Answers map[string]string `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to decode quiz", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	score, err := h.quizService.SubmitQuiz(ctx, userID, quizID, req.Answers)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to submit quiz", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	telemetry.LogWithTrace(ctx).Info("quiz submitted successfully", zap.Int("score", score))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{"score": score})
}

package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sachinggsingh/quiz/internal/model"
	"github.com/sachinggsingh/quiz/internal/service"
	"github.com/sachinggsingh/quiz/internal/telemetry"
	"github.com/sachinggsingh/quiz/internal/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

type CommentHandler struct {
	commentService *service.CommentService
	userService    *service.UserService
}

func NewCommentHandler(commentService *service.CommentService, userService *service.UserService) *CommentHandler {
	return &CommentHandler{
		commentService: commentService,
		userService:    userService,
	}
}

func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("comment-handler").Start(r.Context(), "CreateComment")
	defer span.End()
	// User ID is already set in context by Authenticate middleware
	userIDHex := utils.GetUserId(r.Context())
	if userIDHex == "" {
		err := errors.New("user not authenticated")
		telemetry.LogWithTrace(ctx).Error("unauthenticated access attempt", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "user not authenticated", http.StatusUnauthorized)
		return
	}

	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("invalid user ID", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	// Fetch user to get name
	user, err := h.userService.GetProfile(ctx, userID)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("user not found", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	var comment model.Comment
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to decode comment", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	comment.UserID = userID
	comment.UserName = user.Name

	if err := h.commentService.CreateComment(ctx, &comment); err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to create comment", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	telemetry.CommentCreatedCounter.Add(ctx, 1)
	json.NewEncoder(w).Encode(comment)
}

func (h *CommentHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("comment-handler").Start(r.Context(), "GetComments")
	defer span.End()
	quizIDStr := r.URL.Query().Get("quiz_id")
	var quizID primitive.ObjectID
	if quizIDStr != "" {
		id, err := primitive.ObjectIDFromHex(quizIDStr)
		if err == nil {
			telemetry.LogWithTrace(ctx).Info("fetched quiz id", zap.String("quizID", quizIDStr))
			quizID = id
		}
	}

	comments, err := h.commentService.FindAllComments(ctx, quizID)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to fetch comments", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(comments)
}

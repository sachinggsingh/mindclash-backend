package handler

import (
	"encoding/json"
	"net/http"

	"github.com/sachinggsingh/quiz/internal/service"
	"github.com/sachinggsingh/quiz/internal/telemetry"
	"github.com/sachinggsingh/quiz/internal/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

type RestHandler struct {
	userService *service.UserService
}

func NewRestHandler(userService *service.UserService) *RestHandler {
	return &RestHandler{
		userService: userService,
	}
}

func (h *RestHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("user-handler").Start(r.Context(), "CreateUser")
	defer span.End()

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to decode request body", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("user.email", req.Email))

	user, err := h.userService.CreateUser(ctx, req.Name, req.Email, req.Password)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to create user", zap.Error(err), zap.String("email", req.Email))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	telemetry.UserRegistrationCounter.Add(ctx, 1)

	// Auto-login after registration: generate tokens and set cookies
	accessToken, refreshToken, err := h.userService.Login(ctx, req.Email, req.Password)
	if err != nil {
		// If auto-login fails, still return success for registration
		// but without setting cookies (user will need to sign in manually)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user":    user,
			"message": "User created successfully. Please sign in.",
		})
		return
	}

	// Set cookies (both long-lived: 7 days)
	utils.SetCookie(w, utils.AccessTokenCookieName, accessToken, 7*24*60*60)   // 7 days
	utils.SetCookie(w, utils.RefreshTokenCookieName, refreshToken, 7*24*60*60) // 7 days

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":          user,
		"message":       "Account created and logged in successfully",
		"access_token":  accessToken,  // For backward compatibility
		"refresh_token": refreshToken, // For backward compatibility
	})
}

func (h *RestHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("user-handler").Start(r.Context(), "Login")
	defer span.End()

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to decode login request", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("user.email", req.Email))

	accessToken, refreshToken, err := h.userService.Login(ctx, req.Email, req.Password)
	if err != nil {
		telemetry.LogWithTrace(ctx).Warn("login attempt failed", zap.Error(err), zap.String("email", req.Email))
		span.SetStatus(codes.Error, "authentication failed")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	telemetry.UserLoginCounter.Add(ctx, 1)
	telemetry.LogWithTrace(ctx).Info("user logged in", zap.String("email", req.Email))

	// Set cookies (both long-lived: 7 days)
	utils.SetCookie(w, utils.AccessTokenCookieName, accessToken, 7*24*60*60)   // 7 days
	utils.SetCookie(w, utils.RefreshTokenCookieName, refreshToken, 7*24*60*60) // 7 days

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Login successful",
		// Optionally still return tokens in response for backward compatibility
		// Remove these lines if you want cookies-only authentication
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *RestHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("user-handler").Start(r.Context(), "GetMe")
	defer span.End()

	// User ID is already set in context by Authenticate middleware
	userIDHex := utils.GetUserId(ctx)
	if userIDHex == "" {
		telemetry.LogWithTrace(ctx).Warn("unauthenticated access attempt to GetMe")
		http.Error(w, "user not authenticated", http.StatusUnauthorized)
		return
	}

	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("invalid user ID in context", zap.String("userID", userIDHex))
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("user.id", userIDHex))

	user, err := h.userService.GetProfile(ctx, userID)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("failed to fetch user profile", zap.Error(err), zap.String("userID", userIDHex))
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}

func (h *RestHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("user-handler").Start(r.Context(), "RefreshToken")
	defer span.End()

	// Try to get refresh token from cookie first
	refreshToken, err := utils.GetCookie(r, utils.RefreshTokenCookieName)
	if err != nil {
		// Fallback to request body for backward compatibility
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			telemetry.LogWithTrace(ctx).Error("failed to decode refresh token request", zap.Error(err))
			http.Error(w, "refresh token not provided", http.StatusBadRequest)
			return
		}
		refreshToken = req.RefreshToken
	}

	accessToken, err := h.userService.RefreshToken(ctx, refreshToken)
	if err != nil {
		telemetry.LogWithTrace(ctx).Warn("token refresh failed", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Set new access token cookie (long-lived)
	utils.SetCookie(w, utils.AccessTokenCookieName, accessToken, 7*24*60*60) // 7 days

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":      "Token refreshed successfully",
		"access_token": accessToken, // Optionally return for backward compatibility
	})
}

func (h *RestHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("user-handler").Start(r.Context(), "Logout")
	defer span.End()

	// Clear both cookies
	utils.ClearCookie(w, utils.AccessTokenCookieName)
	utils.ClearCookie(w, utils.RefreshTokenCookieName)

	telemetry.LogWithTrace(ctx).Info("user logged out")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged out successfully",
	})
}

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/sachinggsingh/quiz/config"
	"github.com/sachinggsingh/quiz/internal/model"
	"github.com/sachinggsingh/quiz/internal/repo"
	"github.com/sachinggsingh/quiz/internal/service"
	"github.com/sachinggsingh/quiz/internal/telemetry"
	"github.com/stripe/stripe-go/v84"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

type SubscriptionHandler struct {
	svc      service.SubscriptionService
	subsRepo repo.SubscriptionRepo
}

func NewSubscriptonHandler(svc service.SubscriptionService, subsRepo repo.SubscriptionRepo) *SubscriptionHandler {
	return &SubscriptionHandler{
		svc:      svc,
		subsRepo: subsRepo,
	}
}

type createSubscriptionRequest struct {
	PriceID string `json:"price_id"`
	Email   string `json:"email"`
}
type APIResponse struct {
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Success bool   `json:"success"`
}

// Create handler for the subscription
func (h *SubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("subscription-create").Start(r.Context(), "Create")
	defer span.End()
	var req createSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		telemetry.LogWithTrace(ctx).Error("Invalid body", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		respondWithError(w, http.StatusBadRequest, "Invalid body")
		return
	}
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		err := errors.New("user not authenticated")
		telemetry.LogWithTrace(ctx).Error("User is not authorized", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		respondWithError(w, http.StatusUnauthorized, "User is not authorized")
		return
	}
	span.SetAttributes(attribute.String("user.id", userID))
	session, err := h.svc.CreateSubscription(ctx, userID, req.PriceID, req.Email)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("Failed to create subscription", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Frontend expects a top-level { url } for redirecting to Stripe Checkout
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"url": session.URL,
	})

}

// Get the subscription
func (h *SubscriptionHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("GetSubscription").Start(r.Context(), "GetSubscription")
	defer span.End()
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		err := errors.New("user not authenticated")
		telemetry.LogWithTrace(ctx).Error("User is not authorized", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}
	span.SetAttributes(attribute.String("user.id", userID))

	sub, err := h.svc.GetSubscription(ctx, userID)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("Failed to get subscription", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		respondWithError(w, http.StatusNotFound, "Subscription not found")
		return
	}

	respondWithSuccess(w, http.StatusOK, sub)
}

// cancel subscription
func (h *SubscriptionHandler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("CancelSubscription").Start(r.Context(), "CancelSubscription")
	defer span.End()
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		err := errors.New("user not authenticated")
		telemetry.LogWithTrace(ctx).Error("User is not authorized", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}
	span.SetAttributes(attribute.String("user.id", userID))

	err := h.svc.CancelSubscription(ctx, userID)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("Failed to cancel subscription", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	telemetry.LogWithTrace(ctx).Info("Subscription cancelled", zap.String("userID", userID))
	respondWithSuccess(w, http.StatusOK, map[string]string{"message": "Subscription cancelled"})
}

// webhook
func (h *SubscriptionHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("StripeWebhook").Start(r.Context(), "StripeWebhook")
	defer span.End()
	// 1. Read raw body (CRITICAL for signature verification)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("Failed to read body", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// 2. Verify Stripe signature (tolerant of API version mismatches)
	sigHeader := r.Header.Get("Stripe-Signature")
	stripeClient := config.NewStripeClient()
	event, err := stripeClient.ConstructEvent(body, sigHeader)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("Stripe webhook signature verification failed", zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "Webhook signature verification failed", http.StatusBadRequest)
		return
	}

	// 3. Handle key subscription events only (these carry Subscription objects)
	switch event.Type {
	case "customer.subscription.created",
		"customer.subscription.updated",
		"customer.subscription.deleted":
		var stripeSub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
			telemetry.LogWithTrace(ctx).Error("Failed to parse subscription for event", zap.String("event type", string(event.Type)), zap.Error(err))
			w.WriteHeader(http.StatusOK)
			return
		}
		h.syncSubscription(ctx, &stripeSub)
	}

	telemetry.LogWithTrace(ctx).Info("Stripe webhook processed", zap.String("event type", string(event.Type)))
	// 5. Always return 200
	w.WriteHeader(http.StatusOK)
}

func (h *SubscriptionHandler) syncSubscription(ctx context.Context, stripeSub *stripe.Subscription) {
	ctx, span := otel.Tracer("syncSubscription").Start(ctx, "syncSubscription")
	defer span.End()
	metaDataUserId := ""
	// We store the user ID in subscription metadata when creating the Checkout Session.
	if stripeSub.Metadata != nil {
		metaDataUserId = stripeSub.Metadata["user_id"]
	}
	if metaDataUserId == "" {
		telemetry.LogWithTrace(ctx).Error("No user_id metadata found on subscription", zap.String("subscription_id", stripeSub.ID))
		log.Printf("No user_id metadata found on subscription %s", stripeSub.ID)
		return
	}
	var priceToPlan = map[string]model.Plan{
		config.LoadEnv().STRIPE_PRO_PLAN_PRICE_ID:        model.PlanPro,
		config.LoadEnv().STRIPE_ENTERPRISE_PLAN_PRICE_ID: model.PlanEnterprise,
	}

	var plan model.Plan
	if len(stripeSub.Items.Data) > 0 {
		priceID := stripeSub.Items.Data[0].Price.ID
		if p, exists := priceToPlan[priceID]; exists {
			plan = p
		}
	}

	// Derive subscription period from available Stripe timestamps
	start := time.Unix(stripeSub.StartDate, 0)
	endTs := stripeSub.TrialEnd
	if endTs == 0 {
		endTs = stripeSub.EndedAt
	}
	end := time.Unix(endTs, 0)

	sub := &model.Subscription{
		UserID:                     metaDataUserId,
		StripeSubscriptionID:       stripeSub.ID,
		StripeCustomerID:           stripeSub.Customer.ID,
		Status:                     model.Status(stripeSub.Status),
		Plan:                       plan,
		Subscription_Starting_Date: start,
		Subscription_Ending_Date:   end,
		UpdatedAt:                  time.Now(),
	}

	err := h.subsRepo.CreateOrUpdate(ctx, sub)
	if err != nil {
		telemetry.LogWithTrace(ctx).Error("Failed to sync subscription", zap.String("subscription_id", stripeSub.ID), zap.Error(err))
		log.Printf("Failed to sync subscription %s, %v", stripeSub.ID, err)
		return
	}
	// for testing
	telemetry.LogWithTrace(ctx).Info("Synced subscription", zap.String("subscription_id", stripeSub.ID), zap.String("user_id", metaDataUserId), zap.String("status", string(stripeSub.Status)))
	log.Printf("Synced subscription %s for user %s → status: %s",
		stripeSub.ID, metaDataUserId, stripeSub.Status)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(APIResponse{
		Error:   message,
		Success: false,
	})
}

func respondWithSuccess(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(APIResponse{
		Data:    data,
		Success: true,
	})
}

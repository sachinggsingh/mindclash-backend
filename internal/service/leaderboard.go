package service

import (
	"context"
	"log"
	"time"

	"github.com/sachinggsingh/quiz/internal/model"
	"github.com/sachinggsingh/quiz/internal/repo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LeaderboardEntry is the exact shape sent over WebSocket so the frontend always gets "score" and related fields.
type LeaderboardEntry struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	Email            string               `json:"email"`
	Score            int                  `json:"score"`
	AverageScore     float64              `json:"average_score"`
	CompletedQuizIDs []primitive.ObjectID `json:"completed_quiz_ids"`
}

func userToEntry(u *model.User) LeaderboardEntry {
	id := ""
	if !u.ID.IsZero() {
		id = u.ID.Hex()
	}
	return LeaderboardEntry{
		ID:               id,
		Name:             u.Name,
		Email:            u.Email,
		Score:            u.Score,
		AverageScore:     u.AverageScore,
		CompletedQuizIDs: u.CompletedQuizIDs,
	}
}

// LeaderboardBroadcaster defines how leaderboard updates are pushed to clients.
// This decouples the service layer from any specific transport (WebSocket, SSE, etc.).
type LeaderboardBroadcaster interface {
	BroadcastLeaderboardUpdate(entries []LeaderboardEntry)
}

type LeaderboardService struct {
	userRepo        repo.UserRepo
	leaderboardRepo repo.LeaderboardRepo
	broadcaster     LeaderboardBroadcaster
}

func NewLeaderboardService(userRepo repo.UserRepo, leaderboardRepo repo.LeaderboardRepo, broadcaster LeaderboardBroadcaster) *LeaderboardService {
	return &LeaderboardService{
		userRepo:        userRepo,
		leaderboardRepo: leaderboardRepo,
		broadcaster:     broadcaster,
	}
}

func (ls *LeaderboardService) BroadcastUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Get top user IDs from Redis
	topEntries, err := ls.leaderboardRepo.GetTopUsers(ctx, 10)
	if err != nil {
		log.Printf("Error fetching top entries from Redis: %v", err)
		return
	}

	entries := make([]LeaderboardEntry, 0, len(topEntries))
	for _, entry := range topEntries {
		oid, err := primitive.ObjectIDFromHex(entry.UserID)
		if err != nil {
			continue
		}

		// 2. Hydrate user details from DB
		user, err := ls.userRepo.FindByID(ctx, oid)
		if err != nil {
			log.Printf("Error fetching user details from DB for %s: %v", entry.UserID, err)
			continue
		}

		// Use the score from Redis (source of truth for leaderboard)
		user.Score = int(entry.Score)
		entries = append(entries, userToEntry(user))
	}

	ls.broadcaster.BroadcastLeaderboardUpdate(entries)
}

func (ls *LeaderboardService) Sync() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("Syncing leaderboard from MongoDB to Redis...")
	users, _, err := ls.userRepo.GetTopUsers(ctx, 1, 100) // Sync top 100
	if err != nil {
		log.Printf("Failed to sync leaderboard: %v", err)
		return
	}

	for _, u := range users {
		_ = ls.leaderboardRepo.UpdateScore(ctx, u.UserId, u.Score)
	}
	log.Printf("Leaderboard sync complete: %d users updated", len(users))
}

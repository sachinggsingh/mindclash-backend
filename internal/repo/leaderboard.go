package repo

import (
	"context"
	"github.com/go-redis/redis"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ScoreEntry struct {
	UserID string
	Score  float64
}

type LeaderboardRepo interface {
	UpdateScore(ctx context.Context, userID primitive.ObjectID, score int) error
	GetTopUsers(ctx context.Context, limit int64) ([]ScoreEntry, error)
}

type redisLeaderboardRepo struct {
	client *redis.Client
	key    string
}

func NewLeaderboardRepo(client *redis.Client) LeaderboardRepo {
	return &redisLeaderboardRepo{
		client: client,
		key:    "leaderboard:global",
	}
}

func (r *redisLeaderboardRepo) UpdateScore(ctx context.Context, userID primitive.ObjectID, score int) error {
	// Using ZAdd to set the absolute score for the user in the leaderboard set
	err := r.client.ZAdd(r.key, redis.Z{
		Score:  float64(score),
		Member: userID.Hex(),
	}).Err()
	return err
}

func (r *redisLeaderboardRepo) GetTopUsers(ctx context.Context, limit int64) ([]ScoreEntry, error) {
	// ZRevRangeWithScores returns users sorted by score descending
	results, err := r.client.ZRevRangeWithScores(r.key, 0, limit-1).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]ScoreEntry, 0, len(results))
	for _, z := range results {
		entries = append(entries, ScoreEntry{
			UserID: z.Member.(string),
			Score:  z.Score,
		})
	}
	return entries, nil
}

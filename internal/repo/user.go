package repo

import (
	"context"
	"time"

	"github.com/sachinggsingh/quiz/internal/model"
	"github.com/sachinggsingh/quiz/internal/telemetry"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

type UserRepo interface {
	InitIndexes(ctx context.Context) error
	Create(ctx context.Context, user *model.User) error
	UpdateRefreshToken(ctx context.Context, userID primitive.ObjectID, refreshToken string) error
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*model.User, error)
	UpdateStats(ctx context.Context, userID primitive.ObjectID, score int, completedQuizzes int, averageScore float64, streak int, activity map[string]int, completedQuizIDs []primitive.ObjectID) error
	UpdateScore(ctx context.Context, userID primitive.ObjectID, score int) error
	GetTopUsers(ctx context.Context, page int64, limit int64) ([]model.User, int64, error)
}

type userRepoImpl struct {
	collection *mongo.Collection
}

func NewUserRepo(db *mongo.Database) UserRepo {
	repo := &userRepoImpl{
		collection: db.Collection("users"),
	}
	repo.InitIndexes(context.Background())
	return repo
}

func (r *userRepoImpl) InitIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	indexModels := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "score", Value: -1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexModels)
	return err
}

func (r *userRepoImpl) Create(ctx context.Context, user *model.User) error {
	ctx, span := otel.Tracer("repo").Start(ctx, "UserRepo.Create")
	defer span.End()

	start := time.Now()
	telemetry.DBQueryCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create"), attribute.String("collection", "users")))

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	user.ID = primitive.NewObjectID()
	user.CreatedAt, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	user.UpdatedAt, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	user.UserId = user.ID
	user.Streak = 0
	user.Activity = make(map[string]int)
	user.CompletedQuizIDs = make([]primitive.ObjectID, 0)
	_, err := r.collection.InsertOne(ctx, user)

	duration := time.Since(start).Seconds()
	telemetry.DBQueryDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create"), attribute.String("collection", "users")))

	if err != nil {
		span.RecordError(err)
		telemetry.DBErrorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create"), attribute.String("collection", "users")))
		telemetry.LogWithTrace(ctx).Error("DB Error: UserRepo.Create", zap.Error(err))
		return err
	}

	telemetry.LogWithTrace(ctx).Info("DB Success: UserRepo.Create", zap.String("user_id", user.ID.Hex()))
	return nil
}
func (r *userRepoImpl) UpdateRefreshToken(ctx context.Context, userID primitive.ObjectID, refreshToken string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}
	updatedAt, _ := time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	update := bson.M{"$set": bson.M{
		"refresh_token": refreshToken,
		"updated_at":    updatedAt,
	}}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

// returns the array of details of the user
func (r *userRepoImpl) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	ctx, span := otel.Tracer("repo").Start(ctx, "UserRepo.FindByEmail")
	defer span.End()

	start := time.Now()
	telemetry.DBQueryCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "find_by_email"), attribute.String("collection", "users")))

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var user model.User
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)

	duration := time.Since(start).Seconds()
	telemetry.DBQueryDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "find_by_email"), attribute.String("collection", "users")))

	if err != nil {
		if err == mongo.ErrNoDocuments {
			telemetry.LogWithTrace(ctx).Debug("DB: User not found by email", zap.String("email", email))
			return nil, err
		}
		span.RecordError(err)
		telemetry.DBErrorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "find_by_email"), attribute.String("collection", "users")))
		telemetry.LogWithTrace(ctx).Error("DB Error: UserRepo.FindByEmail", zap.Error(err), zap.String("email", email))
		return nil, err
	}

	telemetry.LogWithTrace(ctx).Debug("DB Success: UserRepo.FindByEmail", zap.String("user_id", user.ID.Hex()))
	return &user, nil
}

// returning the user
func (r *userRepoImpl) FindByID(ctx context.Context, id primitive.ObjectID) (*model.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var user model.User
	err := r.collection.FindOne(ctx, bson.M{"user_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepoImpl) UpdateStats(ctx context.Context, userID primitive.ObjectID, score int, completedQuizzes int, averageScore float64, streak int, activity map[string]int, completedQuizIDs []primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	update := bson.M{"$set": bson.M{
		"score":              score,
		"completed_quizzes":  completedQuizzes,
		"average_score":      averageScore,
		"streak":             streak,
		"activity":           activity,
		"completed_quiz_ids": completedQuizIDs,
		"updated_at":         time.Now(),
	}}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *userRepoImpl) UpdateScore(ctx context.Context, userID primitive.ObjectID, score int) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	updatedAt, _ := time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	update := bson.M{"$set": bson.M{
		"score":      score,
		"updated_at": updatedAt,
	}}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

// GetTopUsers returns a page of users sorted by score descending and the total number of users paginated
func (r *userRepoImpl) GetTopUsers(ctx context.Context, page int64, limit int64) ([]model.User, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	total, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	skip := (page - 1) * limit

	opts := options.Find().SetSort(bson.D{{Key: "score", Value: -1}}).SetSkip(skip).SetLimit(limit)
	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var users []model.User
	if err = cursor.All(ctx, &users); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

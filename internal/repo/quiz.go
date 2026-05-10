package repo

import (
	"context"
	"fmt"
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

type QuizRepo interface {
	Create(ctx context.Context, quiz *model.Quiz) error
	FindByID(ctx context.Context, quiz_id primitive.ObjectID) (*model.Quiz, error)
	FindAll(ctx context.Context) ([]model.Quiz, error)
	FindAllByUser(ctx context.Context, user_id primitive.ObjectID) ([]model.Quiz, error)
	FindByCategory(ctx context.Context, category string) ([]model.Quiz, error)
}

type quizRepo struct {
	collection *mongo.Collection
}

func NewQuizRepo(db *mongo.Database) *quizRepo {
	return &quizRepo{
		collection: db.Collection("quizzes", &options.CollectionOptions{}),
	}
}

func (r *quizRepo) Create(ctx context.Context, quiz *model.Quiz) error {
	ctx, span := otel.Tracer("repo").Start(ctx, "QuizRepo.Create")
	defer span.End()

	start := time.Now()
	telemetry.DBQueryCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create"), attribute.String("collection", "quizzes")))

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	quiz.ID = primitive.NewObjectID()
	quiz.CreatedAt, _ = time.Parse(time.RFC1123, time.Now().Format(time.RFC1123))
	quiz.UpdatedAt, _ = time.Parse(time.RFC1123, time.Now().Format(time.RFC1123))
	if quiz.QuizID.IsZero() {
		quiz.QuizID = quiz.ID
	}
	for i := range quiz.Questions {
		if quiz.Questions[i].ID.IsZero() {
			quiz.Questions[i].ID = primitive.NewObjectID()
		}
	}
	_, err := r.collection.InsertOne(ctx, quiz)

	duration := time.Since(start).Seconds()
	telemetry.DBQueryDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create"), attribute.String("collection", "quizzes")))

	if err != nil {
		span.RecordError(err)
		telemetry.DBErrorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create"), attribute.String("collection", "quizzes")))
		telemetry.LogWithTrace(ctx).Error("DB Error: QuizRepo.Create", zap.Error(err))
		return err
	}

	telemetry.LogWithTrace(ctx).Info("DB Success: QuizRepo.Create", zap.String("quiz_id", quiz.ID.Hex()))
	return nil
}

func (r *quizRepo) FindByID(ctx context.Context, quiz_id primitive.ObjectID) (*model.Quiz, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var rawResult bson.M
	
	// Try to find by quiz_id first, then fallback to _id
	err := r.collection.FindOne(ctx, bson.M{"quiz_id": quiz_id}).Decode(&rawResult)
	if err != nil {
		// Fallback to _id if quiz_id doesn't match
		err = r.collection.FindOne(ctx, bson.M{"_id": quiz_id}).Decode(&rawResult)
		if err != nil {
			return nil, err
		}
	}
	
	// Debug: Check raw MongoDB document
	fmt.Printf("Raw MongoDB document keys: %v\n", getKeys(rawResult))
	if questionsRaw, ok := rawResult["questions"]; ok {
		fmt.Printf("Raw questions from MongoDB (type: %T)\n", questionsRaw)
		if questionsArray, ok := questionsRaw.(bson.A); ok {
			fmt.Printf("Questions array length: %d\n", len(questionsArray))
			if len(questionsArray) > 0 {
				if firstQ, ok := questionsArray[0].(bson.M); ok {
					fmt.Printf("First question raw keys: %v\n", getKeys(firstQ))
					fmt.Printf("First question raw data: %+v\n", firstQ)
				}
			}
		} else {
			fmt.Printf("WARNING: questions is not bson.A, it's type: %T, value: %v\n", questionsRaw, questionsRaw)
		}
	} else {
		fmt.Printf("WARNING: 'questions' field not found in raw MongoDB document\n")
	}
	
	// Decode raw result into Quiz struct
	var quiz model.Quiz
	bytes, marshalErr := bson.Marshal(rawResult)
	if marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal raw result: %v", marshalErr)
	}
	
	err = bson.Unmarshal(bytes, &quiz)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal quiz: %v", err)
	}
	
	// Debug: Log questions count after decode
	fmt.Printf("Quiz decoded - ID: %s, Title: %s, Questions count: %d\n", quiz.ID.Hex(), quiz.Title, len(quiz.Questions))
	if len(quiz.Questions) > 0 {
		for i, q := range quiz.Questions {
			fmt.Printf("Question %d - ID: %s, Text: '%s', Options count: %d, Options: %v, Answer: %d\n", 
				i, q.ID.Hex(), q.Text, len(q.Options), q.Options, q.Answer)
		}
	} else {
		fmt.Printf("WARNING: Questions array is empty after decode\n")
	}
	
	return &quiz, nil
}

// Helper function to get keys from a bson.M
func getKeys(m bson.M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (r *quizRepo) FindAll(ctx context.Context) ([]model.Quiz, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var quizzes []model.Quiz
	if err = cursor.All(ctx, &quizzes); err != nil {
		return nil, err
	}
	return quizzes, nil
}

func (r *quizRepo) FindAllByUser(ctx context.Context, user_id primitive.ObjectID) ([]model.Quiz, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{"user_id": user_id})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var quizzes []model.Quiz
	if err = cursor.All(ctx, &quizzes); err != nil {
		return nil, err
	}
	return quizzes, nil
}

func (r *quizRepo) FindByCategory(ctx context.Context, category string) ([]model.Quiz, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{"category": category})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var quizzes []model.Quiz
	if err = cursor.All(ctx, &quizzes); err != nil {
		return nil, err
	}
	return quizzes, nil
}

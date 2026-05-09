package main

import (
	"context"
	"log"

	"github.com/sachinggsingh/quiz/config"
	"github.com/sachinggsingh/quiz/internal/api"
	"github.com/sachinggsingh/quiz/internal/telemetry"
)

func main() {
	// 1. Config & DB
	env := config.LoadEnv()
	db, err := config.ConnectDB(env.MONGO_URI, env.DB_NAME)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}

	// Initialize  Telemetry
	// Logger
	telemetry.InitLogger()
	defer telemetry.SyncLogger()

	// Metrics
	metricsShutdown := telemetry.InitMetrics()
	defer func() {
		if err := metricsShutdown(context.Background()); err != nil {
			log.Println(err)
		}
	}()

	// Tracer
	shutdown, err := telemetry.InitTracer()
	if err != nil {
		log.Fatalf("Failed to initialize Tracer: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			log.Fatal(err)
		}
	}()

	// Initialize Gemini
	if err := config.InitGemini(context.Background(), env.GEMINI_API_KEY, env.GEMINI_MODEL); err != nil {
		log.Fatalf("Failed to initialize Gemini: %v", err)
	}

	// 2. Initialize and Run Server
	server := api.NewServer(db.DB, env)
	if err := server.Run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sachinggsingh/quiz/config"
	"github.com/sachinggsingh/quiz/internal/service"
)

func main() {
	env := config.LoadEnv()
	if err := config.InitGemini(context.Background(), env.GEMINI_API_KEY, env.GEMINI_MODEL); err != nil {
		log.Fatalf("Failed to initialize Gemini: %v", err)
	}

	quizService := service.NewQuizService(nil, nil, nil, nil, nil) // Mock repos for pure generation test

	quiz, err := quizService.GenerateQuiz(
		context.Background(),
		"Golang Basics",
		"Programming",
		"Easy",
		"A beginner level quiz on Go programming language features.",
		3,
		30,
	)

	if err != nil {
		log.Fatalf("Failed to generate quiz: %v", err)
	}

	fmt.Printf("Generated Quiz: %+v\n", quiz)
}

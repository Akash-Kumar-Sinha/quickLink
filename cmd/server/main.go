package main

import (
	"flag"
	"log"

	"quicklink/internal/app"
	"quicklink/internal/config"
)

func main() {
	port := flag.String("port", "", "Port for the HTTP server to listen on")
	host := flag.String("host", "", "Host/interface for the HTTP server to bind")
	baseURL := flag.String("base-url", "", "Base public URL for shortened links")
	databaseURL := flag.String("db", "", "PostgreSQL connection URL")
	redisURL := flag.String("redis", "", "Redis connection URL")
	flag.Parse()

	cfg := config.New(
		config.WithPort(*port),
		config.WithHost(*host),
		config.WithBaseURL(*baseURL),
		config.WithDatabaseURL(*databaseURL),
		config.WithRedisURL(*redisURL),
	)

	server, err := app.New(
		app.WithConfig(cfg),
	)
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	if err := server.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

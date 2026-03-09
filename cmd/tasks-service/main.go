package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"tasks-service/internal/app"
	"tasks-service/internal/config"
)

func main() {
	cfg := config.FromEnv()

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("service stopped with error: %v", err)
	}
}

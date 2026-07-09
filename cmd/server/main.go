package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/yourorg/agent_customer_service/internal/bootstrap"
	"github.com/yourorg/agent_customer_service/internal/config"
)

func main() {
	_ = godotenv.Load()

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}

	cfg, err := config.Load(env)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	app, cleanup, err := bootstrap.NewApp(ctx, cfg)
	if err != nil {
		log.Fatalf("bootstrap app: %v", err)
	}
	defer cleanup()

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler: app.Router,
	}

	go func() {
		log.Printf("server listening on %s:%s (env=%s)", cfg.Server.Host, cfg.Server.Port, env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}

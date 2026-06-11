// Command babyapp starts the BabyApp API server.
package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"babyapp/backend/internal/config"
	"babyapp/backend/internal/handlers"
	"babyapp/backend/internal/repository"
)

func main() {
	cfg := config.Load()

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	db, err := repository.Connect(ctx, cfg.MongoURI, cfg.DBName)
	cancel()
	if err != nil {
		log.Fatalf("❌ MongoDB: %v", err)
	}
	log.Printf("✅ MongoDB conectado: %s / %s", redactMongoURI(cfg.MongoURI), cfg.DBName)

	// Ensure all indexes exist (idempotent)
	idxCtx, idxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	db.EnsureIndexes(idxCtx)
	idxCancel()

	// Build router
	router, err := handlers.NewRouter(cfg, db)
	if err != nil {
		log.Fatalf("❌ Router: %v", err)
	}

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("🚀 Servidor en http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("servidor: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("⏳ Apagando servidor...")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	if err := db.Disconnect(shutCtx); err != nil {
		log.Printf("mongo disconnect: %v", err)
	}
	log.Println("✅ Servidor apagado correctamente")
}

func redactMongoURI(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw
	}
	parsed.User = nil
	return parsed.String()
}

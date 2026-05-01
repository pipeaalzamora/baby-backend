// Package config loads application configuration from environment variables.
package config

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	MongoURI    string
	DBName      string
	JWTSecret   string
	JWTExpiry   time.Duration
	Port        string
	FrontendURL string
	BaseURL     string
	UploadDir   string
}

// Load reads configuration from .env file and environment variables.
// Environment variables take precedence over .env file values.
func Load() *Config {
	// Best-effort load of .env — ignore error if file doesn't exist
	_ = godotenv.Load()

	return &Config{
		MongoURI:    getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DBName:      getEnv("DB_NAME", "babyapp"),
		JWTSecret:   getEnv("JWT_SECRET", "babyapp-dev-secret-change-in-prod"),
		JWTExpiry:   30 * 24 * time.Hour, // 30 days
		Port:        getEnv("PORT", "3001"),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:5173"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:3001"),
		UploadDir:   getEnv("UPLOAD_DIR", "./uploads"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

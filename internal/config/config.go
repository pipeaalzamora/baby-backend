// Package config loads application configuration from environment variables.
package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	MongoURI          string
	DBName            string
	Port              string
	FrontendURL       string
	BaseURL           string
	UploadDir         string
	FirebaseProjectID string
	AWSRegion         string
	S3PhotosBucket    string
	S3PresignTTL      int
	TheMealDBAPIKey   string
}

// Load reads configuration from .env file and environment variables.
// Environment variables take precedence over .env file values.
func Load() *Config {
	// Best-effort load of .env — ignore error if file doesn't exist
	_ = godotenv.Load()

	firebaseProjectID := getEnv("FIREBASE_PROJECT_ID", "proyectos-hobbys-495300")
	if firebaseProjectID == "" {
		log.Fatal("FIREBASE_PROJECT_ID no está configurado")
	}

	return &Config{
		MongoURI:          getEnv("MONGO_URI", "mongodb://localhost:27017"),
		DBName:            getEnv("DB_NAME", "babyapp"),
		Port:              getEnv("PORT", "3001"),
		FrontendURL:       getEnv("FRONTEND_URL", "http://localhost:4200"),
		BaseURL:           getEnv("BASE_URL", "http://localhost:3001"),
		UploadDir:         getEnv("UPLOAD_DIR", "./uploads"),
		FirebaseProjectID: firebaseProjectID,
		AWSRegion:         getEnv("AWS_REGION", "sa-east-1"),
		S3PhotosBucket:    getEnv("S3_PHOTOS_BUCKET", ""),
		S3PresignTTL:      getEnvInt("S3_PRESIGN_TTL_SECONDS", 900),
		TheMealDBAPIKey:   getEnv("THEMEALDB_API_KEY", "1"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		parsed, err := strconv.Atoi(v)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

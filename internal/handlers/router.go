package handlers

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"babyapp/backend/internal/config"
	"babyapp/backend/internal/middleware"
	"babyapp/backend/internal/repository"
	"babyapp/backend/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// NewRouter builds and returns the configured Gin engine.
func NewRouter(cfg *config.Config, db *repository.DB) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// CORS: los origins deben coincidir exactamente con el browser Origin.
	allowedOrigins := corsAllowedOrigins(cfg)
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Serve uploaded files
	r.Static("/uploads", cfg.UploadDir)

	s3Service, err := storage.NewS3Service(
		context.Background(),
		cfg.AWSRegion,
		cfg.S3PhotosBucket,
		time.Duration(cfg.S3PresignTTL)*time.Second,
	)
	if err != nil {
		return nil, err
	}

	// Health check — no auth required
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "ts": time.Now().UTC()})
	})

	firebaseAuth := middleware.NewFirebaseVerifier(cfg.FirebaseProjectID)

	// ─── Protected routes ─────────────────────────────────────────────────
	authMW := middleware.RequireFirebaseAuth(db, firebaseAuth)
	api := r.Group("/api", authMW)

	// Auth session
	authH := NewAuthHandler(db, s3Service)
	api.GET("/auth/me", authH.Me)
	api.DELETE("/auth/account", authH.DeleteAccount)

	// Child
	childH := NewChildHandler(db, s3Service)
	api.GET("/child", childH.Get)
	api.POST("/child", childH.Upsert)
	api.GET("/children", childH.List)
	api.POST("/children", childH.Create)
	api.PATCH("/children/:id", childH.Update)
	api.POST("/children/:id/select", childH.Select)
	api.POST("/children/:id/photo/presign", childH.PresignPhoto)

	// Vaccines
	vaccineH := NewVaccineHandler(db)
	api.GET("/vaccines", vaccineH.List)
	api.POST("/vaccines/seed-local", vaccineH.SeedLocal)
	api.POST("/vaccines/bulk", vaccineH.BulkCreate)
	api.PATCH("/vaccines/:id", vaccineH.MarkAdministered) // PATCH es semánticamente correcto para actualización parcial

	// Measurements
	measureH := NewMeasurementHandler(db)
	api.GET("/measurements", measureH.List)
	api.POST("/measurements", measureH.Create)
	api.DELETE("/measurements/:id", measureH.Delete)

	// Checkups
	checkupH := NewCheckupHandler(db)
	api.GET("/checkups", checkupH.List)
	api.POST("/checkups", checkupH.Create)
	api.PATCH("/checkups/:id", checkupH.Patch)
	api.DELETE("/checkups/:id", checkupH.Delete)

	// Milestones
	milestoneH := NewMilestoneHandler(db)
	api.GET("/milestones", milestoneH.List)
	api.POST("/milestones", milestoneH.Create)
	api.DELETE("/milestones/:id", milestoneH.Delete)

	developmentH := NewDevelopmentHandler(db)
	api.GET("/development/advice", developmentH.Advice)

	// Diary
	diaryH := NewDiaryHandler(db)
	api.GET("/diary", diaryH.List)
	api.POST("/diary", diaryH.Create)
	api.DELETE("/diary/:id", diaryH.Delete)

	// Medications
	medH := NewMedicationHandler(db)
	api.GET("/medications", medH.List)
	api.POST("/medications", medH.Create)
	api.PATCH("/medications/:id", medH.Patch)
	api.DELETE("/medications/:id", medH.Delete)

	// Photos
	photoH := NewPhotoHandler(db, cfg.UploadDir, cfg.BaseURL, s3Service)
	api.GET("/photos", photoH.List)
	api.POST("/photos/presign", photoH.Presign)
	api.POST("/photos", photoH.Upload)
	api.DELETE("/photos/:id", photoH.Delete)

	// Recipes + food introductions
	recipeH := NewRecipeHandler(db, cfg.TheMealDBAPIKey)
	api.GET("/recipes", recipeH.List)
	api.POST("/recipes", recipeH.Create)
	api.PATCH("/recipes/:id/favorite", recipeH.ToggleFavorite)
	api.GET("/recipes/search-external", recipeH.SearchExternal)
	api.GET("/recipes/introductions", recipeH.ListIntroductions)
	api.POST("/recipes/introductions", recipeH.CreateIntroduction)

	// Notifications
	notifH := NewNotificationHandler(db)
	api.GET("/notifications", notifH.List)
	api.PATCH("/notifications/read-all", notifH.MarkAllRead)
	api.PATCH("/notifications/:id/read", notifH.MarkRead)

	// Caregivers
	cgH := NewCaregiverHandler(db)
	api.GET("/caregivers", cgH.List)
	api.POST("/caregivers", cgH.Invite)
	api.DELETE("/caregivers/:id", cgH.Remove)
	api.GET("/caregivers/accept/:token", cgH.AcceptInvite)

	// Chile public-health sources
	externalH := NewExternalHandler()
	api.GET("/external/farmacies", externalH.Pharmacies)
	api.GET("/external/health-centers", externalH.HealthCenters)
	api.GET("/external/medicine-registry", externalH.MedicineRegistry)

	return r, nil
}

func corsAllowedOrigins(cfg *config.Config) []string {
	origins := []string{
		"http://localhost:4200",
		"http://127.0.0.1:4200",
	}
	for _, origin := range cfg.FrontendURLs {
		addOrigin(&origins, origin)
	}
	addOrigin(&origins, cfg.FrontendURL)
	return origins
}

func addOrigin(origins *[]string, origin string) {
	for _, part := range strings.Split(origin, ",") {
		value := strings.TrimRight(strings.TrimSpace(part), "/")
		if value == "" || slices.Contains(*origins, value) {
			continue
		}
		*origins = append(*origins, value)
	}
}

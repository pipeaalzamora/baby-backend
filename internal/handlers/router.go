package handlers

import (
	"net/http"
	"time"

	"babyapp/backend/internal/config"
	"babyapp/backend/internal/middleware"
	"babyapp/backend/internal/repository"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// NewRouter builds and returns the configured Gin engine.
func NewRouter(cfg *config.Config, db *repository.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// CORS — en desarrollo permite localhost:4200, en producción usa FRONTEND_URL.
	allowedOrigins := []string{cfg.FrontendURL}
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

	// Health check — no auth required
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "ts": time.Now().UTC()})
	})

	// ─── Auth routes (no JWT required) ────────────────────────────────────
	authH := NewAuthHandler(db, cfg.JWTSecret, cfg.JWTExpiry)
	googleH := NewGoogleAuthHandler(db, cfg.JWTSecret, cfg.JWTExpiry, cfg.GoogleClientID)
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", authH.Register)
		auth.POST("/login", authH.Login)
		auth.POST("/google", googleH.GoogleLogin)
	}

	// ─── Protected routes ─────────────────────────────────────────────────
	authMW := middleware.RequireAuth(cfg.JWTSecret)
	api := r.Group("/api", authMW)

	// Child
	childH := NewChildHandler(db)
	api.GET("/child", childH.Get)
	api.POST("/child", childH.Upsert)

	// Vaccines
	vaccineH := NewVaccineHandler(db)
	api.GET("/vaccines", vaccineH.List)
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
	api.DELETE("/checkups/:id", checkupH.Delete)

	// Milestones
	milestoneH := NewMilestoneHandler(db)
	api.GET("/milestones", milestoneH.List)
	api.POST("/milestones", milestoneH.Create)
	api.DELETE("/milestones/:id", milestoneH.Delete)

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
	photoH := NewPhotoHandler(db, cfg.UploadDir, cfg.BaseURL)
	api.GET("/photos", photoH.List)
	api.POST("/photos", photoH.Upload)
	api.DELETE("/photos/:id", photoH.Delete)

	// Recipes + food introductions
	recipeH := NewRecipeHandler(db)
	api.GET("/recipes", recipeH.List)
	api.POST("/recipes", recipeH.Create)
	api.PATCH("/recipes/:id/favorite", recipeH.ToggleFavorite)
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

	return r
}

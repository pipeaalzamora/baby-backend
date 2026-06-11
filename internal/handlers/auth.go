// Package handlers contains all Gin route handlers.
package handlers

import (
	"context"
	"net/http"
	"time"

	"babyapp/backend/internal/middleware"
	"babyapp/backend/internal/models"
	"babyapp/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// AuthHandler exposes the authenticated Firebase-backed user session.
type AuthHandler struct{ db *repository.DB }

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(db *repository.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

// Me godoc — GET /api/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario inválido"})
		return
	}

	col := h.db.Collection(repository.ColUsers)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var user models.User
	if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&user); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario no encontrado"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      user.ID.Hex(),
		"email":   user.Email,
		"name":    user.Name,
		"childId": user.ChildID,
		"picture": user.Picture,
	})
}

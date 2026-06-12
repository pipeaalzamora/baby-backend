// Package handlers contains all Gin route handlers.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"babyapp/backend/internal/middleware"
	"babyapp/backend/internal/models"
	"babyapp/backend/internal/repository"
	"babyapp/backend/internal/storage"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// AuthHandler exposes the authenticated Firebase-backed user session.
type AuthHandler struct {
	db *repository.DB
	s3 *storage.S3Service
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(db *repository.DB, s3Service *storage.S3Service) *AuthHandler {
	return &AuthHandler{db: db, s3: s3Service}
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

// DeleteAccount godoc — DELETE /api/auth/account
func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)
	userObjectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario inválido"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if h.s3 != nil && h.s3.Enabled() {
		prefix := fmt.Sprintf("accounts/%s/", safePathSegment(userID))
		if err := h.s3.DeletePrefix(ctx, prefix); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudieron eliminar las fotos de S3"})
			return
		}
	}

	childIDs, err := h.ownedChildIDs(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudieron resolver los perfiles infantiles"})
		return
	}
	if err := h.deleteAccountData(ctx, userObjectID, userID, childIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo eliminar la cuenta"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) ownedChildIDs(ctx context.Context, userID string) ([]string, error) {
	cursor, err := h.db.Collection(repository.ColChildren).Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var children []models.Child
	if err := cursor.All(ctx, &children); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(children))
	for _, child := range children {
		if !child.ID.IsZero() {
			ids = append(ids, child.ID.Hex())
		}
	}
	return ids, nil
}

func (h *AuthHandler) deleteAccountData(ctx context.Context, userObjectID bson.ObjectID, userID string, childIDs []string) error {
	if len(childIDs) > 0 {
		childFilter := bson.M{"childId": bson.M{"$in": childIDs}}
		for _, collection := range []string{
			repository.ColVaccines,
			repository.ColMeasurements,
			repository.ColCheckups,
			repository.ColMilestones,
			repository.ColDiary,
			repository.ColMedications,
			repository.ColPhotos,
			repository.ColRecipes,
			repository.ColFoodIntroductions,
			repository.ColCaregivers,
		} {
			if _, err := h.db.Collection(collection).DeleteMany(ctx, childFilter); err != nil {
				return err
			}
		}
	}

	if _, err := h.db.Collection(repository.ColNotifications).DeleteMany(ctx, bson.M{"userId": userID}); err != nil {
		return err
	}
	if _, err := h.db.Collection(repository.ColChildren).DeleteMany(ctx, bson.M{"userId": userID}); err != nil {
		return err
	}
	if _, err := h.db.Collection(repository.ColUsers).DeleteOne(ctx, bson.M{"_id": userObjectID}); err != nil {
		return err
	}
	return nil
}

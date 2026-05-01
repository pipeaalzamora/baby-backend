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
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// CaregiverHandler manages shared access to child data.
type CaregiverHandler struct{ db *repository.DB }

// NewCaregiverHandler creates a new CaregiverHandler.
func NewCaregiverHandler(db *repository.DB) *CaregiverHandler {
	return &CaregiverHandler{db: db}
}

// List godoc — GET /api/caregivers
func (h *CaregiverHandler) List(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	col := h.db.Collection(repository.ColCaregivers)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, bson.M{"childId": childID},
		options.Find().SetSort(bson.D{{Key: "invitedAt", Value: -1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.Caregiver
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.Caregiver{}
	}
	c.JSON(http.StatusOK, items)
}

// Invite godoc — POST /api/caregivers
func (h *CaregiverHandler) Invite(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)

	var body struct {
		Email string              `json:"email" binding:"required,email"`
		Name  string              `json:"name"`
		Role  models.CaregiverRole `json:"role"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Role == "" {
		body.Role = models.RoleViewer
	}
	if body.Name == "" {
		body.Name = body.Email
	}

	token := randomHex(24)
	caregiver := models.Caregiver{
		ChildID:     childID,
		Email:       body.Email,
		Name:        body.Name,
		Role:        body.Role,
		InvitedAt:   time.Now().Format(time.RFC3339),
		InviteToken: token,
		CreatedAt:   time.Now(),
	}

	col := h.db.Collection(repository.ColCaregivers)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	res, err := col.InsertOne(ctx, caregiver)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	caregiver.ID = res.InsertedID.(bson.ObjectID)

	// Return invite link (in production: send via email)
	c.JSON(http.StatusCreated, gin.H{
		"caregiver":  caregiver,
		"inviteLink": "/invite/" + token,
	})
}

// Remove godoc — DELETE /api/caregivers/:id
func (h *CaregiverHandler) Remove(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	col := h.db.Collection(repository.ColCaregivers)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if _, err := col.DeleteOne(ctx, bson.M{"_id": id}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AcceptInvite godoc — GET /api/caregivers/accept/:token
func (h *CaregiverHandler) AcceptInvite(c *gin.Context) {
	token := c.Param("token")
	col := h.db.Collection(repository.ColCaregivers)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var cg models.Caregiver
	err := col.FindOneAndUpdate(ctx,
		bson.M{"inviteToken": token},
		bson.M{"$set": bson.M{
			"acceptedAt":  time.Now().Format(time.RFC3339),
			"inviteToken": "",
		}},
		opts,
	).Decode(&cg)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invitación no encontrada o ya usada"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "childId": cg.ChildID})
}

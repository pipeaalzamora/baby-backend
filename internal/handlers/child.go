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
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ChildHandler manages the baby profile.
type ChildHandler struct{ db *repository.DB }

// NewChildHandler creates a new ChildHandler.
func NewChildHandler(db *repository.DB) *ChildHandler { return &ChildHandler{db: db} }

// Get godoc — GET /api/child
func (h *ChildHandler) Get(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)
	col := h.db.Collection(repository.ColChildren)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var child models.Child
	if err := col.FindOne(ctx, bson.M{"userId": userID}).Decode(&child); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "perfil no encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, child)
}

// Upsert godoc — POST /api/child
func (h *ChildHandler) Upsert(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)

	var body models.Child
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	col := h.db.Collection(repository.ColChildren)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// $set solo contiene los campos editables — nunca createdAt
	// $setOnInsert solo se aplica en INSERT, evitando el conflicto de MongoDB
	setFields := bson.M{
		"userId":        userID,
		"name":          body.Name,
		"birthDate":     body.BirthDate,
		"gender":        body.Gender,
		"birthWeightKg": body.BirthWeightKg,
		"birthHeightCm": body.BirthHeightCm,
		"updatedAt":     time.Now(),
	}
	if body.BloodType != "" {
		setFields["bloodType"] = body.BloodType
	}
	if body.PhotoURL != "" {
		setFields["photoUrl"] = body.PhotoURL
	}

	filter := bson.M{"userId": userID}
	update := bson.M{
		"$set":         setFields,
		"$setOnInsert": bson.M{"createdAt": time.Now()},
	}
	opts := options.UpdateOne().SetUpsert(true)

	res, err := col.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Devolver el documento actualizado/insertado
	var child models.Child
	var findFilter bson.M
	if res.UpsertedID != nil {
		findFilter = bson.M{"_id": res.UpsertedID}
	} else {
		findFilter = bson.M{"userId": userID}
	}
	_ = col.FindOne(ctx, findFilter).Decode(&child)
	c.JSON(http.StatusOK, child)
}

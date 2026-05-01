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

// MedicationHandler manages prescribed medications.
type MedicationHandler struct{ db *repository.DB }

// NewMedicationHandler creates a new MedicationHandler.
func NewMedicationHandler(db *repository.DB) *MedicationHandler {
	return &MedicationHandler{db: db}
}

// List godoc — GET /api/medications
func (h *MedicationHandler) List(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	col := h.db.Collection(repository.ColMedications)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, bson.M{"childId": childID},
		options.Find().SetSort(bson.D{{Key: "startDate", Value: -1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.Medication
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.Medication{}
	}
	c.JSON(http.StatusOK, items)
}

// Create godoc — POST /api/medications
func (h *MedicationHandler) Create(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	var body models.Medication
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.ChildID = childID
	body.Active = true
	body.CreatedAt = time.Now()

	col := h.db.Collection(repository.ColMedications)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	res, err := col.InsertOne(ctx, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	body.ID = res.InsertedID.(bson.ObjectID)
	c.JSON(http.StatusCreated, body)
}

// Patch godoc — PATCH /api/medications/:id
// Used to deactivate a medication (set active=false, endDate).
func (h *MedicationHandler) Patch(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	col := h.db.Collection(repository.ColMedications)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var med models.Medication
	if err := col.FindOneAndUpdate(ctx, bson.M{"_id": id},
		bson.M{"$set": body}, opts).Decode(&med); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "medicamento no encontrado"})
		return
	}
	c.JSON(http.StatusOK, med)
}

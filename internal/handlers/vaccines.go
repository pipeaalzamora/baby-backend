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

// VaccineHandler manages the PNI vaccine schedule.
type VaccineHandler struct{ db *repository.DB }

// NewVaccineHandler creates a new VaccineHandler.
func NewVaccineHandler(db *repository.DB) *VaccineHandler { return &VaccineHandler{db: db} }

// List godoc — GET /api/vaccines
func (h *VaccineHandler) List(c *gin.Context) {
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)

	col := h.db.Collection(repository.ColVaccines)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, bson.M{"childId": childID},
		options.Find().SetSort(bson.D{{Key: "scheduledDate", Value: 1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var vaccines []models.Vaccine
	if err := cursor.All(ctx, &vaccines); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if vaccines == nil {
		vaccines = []models.Vaccine{}
	}
	c.JSON(http.StatusOK, vaccines)
}

// MarkAdministered godoc — POST /api/vaccines/:id
func (h *VaccineHandler) MarkAdministered(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var body struct {
		AdministeredDate string `json:"administeredDate"`
		Location         string `json:"location"`
		BatchLot         string `json:"batchLot"`
		Reactions        string `json:"reactions"`
		Notes            string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	col := h.db.Collection(repository.ColVaccines)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": bson.M{
		"status":           models.VaccineAdministered,
		"administeredDate": body.AdministeredDate,
		"location":         body.Location,
		"batchLot":         body.BatchLot,
		"reactions":        body.Reactions,
		"notes":            body.Notes,
	}}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var vaccine models.Vaccine
	if err := col.FindOneAndUpdate(ctx, bson.M{"_id": id}, update, opts).Decode(&vaccine); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "vacuna no encontrada"})
		return
	}
	c.JSON(http.StatusOK, vaccine)
}

// BulkCreate godoc — POST /api/vaccines/bulk
// Used to seed the PNI schedule for a new child.
func (h *VaccineHandler) BulkCreate(c *gin.Context) {
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
	if childID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perfil del bebé no encontrado, crea el perfil primero"})
		return
	}

	var vaccines []models.Vaccine
	if err := c.ShouldBindJSON(&vaccines); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	col := h.db.Collection(repository.ColVaccines)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	docs := make([]interface{}, len(vaccines))
	for i, v := range vaccines {
		v.ChildID = childID
		v.CreatedAt = time.Now()
		docs[i] = v
	}

	if _, err := col.InsertMany(ctx, docs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"inserted": len(docs)})
}

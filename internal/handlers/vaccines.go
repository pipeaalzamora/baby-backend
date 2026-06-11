package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"babyapp/backend/internal/healthdata"
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
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
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
	if err := col.FindOneAndUpdate(ctx, bson.M{"_id": id, "childId": childID}, update, opts).Decode(&vaccine); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "vacuna no encontrada"})
		return
	}
	c.JSON(http.StatusOK, vaccine)
}

// SeedLocal godoc — POST /api/vaccines/seed-local
// Seeds the local Chilean PNI infant schedule from the active child's birth date.
func (h *VaccineHandler) SeedLocal(c *gin.Context) {
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
	if childID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perfil del bebé no encontrado"})
		return
	}

	id, err := bson.ObjectIDFromHex(childID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de perfil inválido"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var child models.Child
	if err := h.db.Collection(repository.ColChildren).FindOne(ctx, bson.M{"_id": id}).Decode(&child); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "perfil no encontrado"})
		return
	}

	vaccines, err := healthdata.GenerateInfantPNISchedule(childID, child.BirthDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fecha de nacimiento inválida"})
		return
	}

	inserted, matched, skipped, err := h.upsertVaccines(ctx, childID, vaccines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"inserted": inserted,
		"matched":  matched,
		"skipped":  skipped,
		"source":   healthdata.PNIScheduleSource,
		"version":  healthdata.PNIScheduleVersion,
		"url":      healthdata.PNIScheduleURL,
	})
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	inserted, matched, skipped, err := h.upsertVaccines(ctx, childID, vaccines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"inserted": inserted,
		"matched":  matched,
		"skipped":  skipped,
	})
}

func (h *VaccineHandler) upsertVaccines(ctx context.Context, childID string, vaccines []models.Vaccine) (int64, int64, int, error) {
	col := h.db.Collection(repository.ColVaccines)
	now := time.Now()
	seen := map[string]bool{}
	inserted := int64(0)
	matched := int64(0)
	skipped := 0

	for _, v := range vaccines {
		code := strings.TrimSpace(v.Code)
		scheduledDate := strings.TrimSpace(v.ScheduledDate)
		if code == "" || scheduledDate == "" {
			skipped++
			continue
		}

		key := code + "|" + scheduledDate
		if seen[key] {
			skipped++
			continue
		}
		seen[key] = true

		status := v.Status
		if status == "" {
			status = models.VaccinePending
		}

		filter := bson.M{
			"childId":       childID,
			"code":          code,
			"scheduledDate": scheduledDate,
		}
		update := bson.M{
			"$set": bson.M{
				"name":            strings.TrimSpace(v.Name),
				"ageLabel":        strings.TrimSpace(v.AgeLabel),
				"source":          strings.TrimSpace(v.Source),
				"scheduleVersion": strings.TrimSpace(v.ScheduleVersion),
			},
			"$setOnInsert": bson.M{
				"childId":       childID,
				"code":          code,
				"scheduledDate": scheduledDate,
				"status":        status,
				"createdAt":     now,
			},
		}
		res, err := col.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
		if err != nil {
			return inserted, matched, skipped, err
		}
		inserted += res.UpsertedCount
		matched += res.MatchedCount
	}

	return inserted, matched, skipped, nil
}

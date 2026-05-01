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

// ─── Measurements ─────────────────────────────────────────────────────────────

// MeasurementHandler manages growth measurements.
type MeasurementHandler struct{ db *repository.DB }

// NewMeasurementHandler creates a new MeasurementHandler.
func NewMeasurementHandler(db *repository.DB) *MeasurementHandler {
	return &MeasurementHandler{db: db}
}

// List godoc — GET /api/measurements
func (h *MeasurementHandler) List(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	col := h.db.Collection(repository.ColMeasurements)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, bson.M{"childId": childID},
		options.Find().SetSort(bson.D{{Key: "date", Value: 1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.Measurement
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.Measurement{}
	}
	c.JSON(http.StatusOK, items)
}

// Create godoc — POST /api/measurements
func (h *MeasurementHandler) Create(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	var body models.Measurement
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.ChildID = childID
	body.CreatedAt = time.Now()

	col := h.db.Collection(repository.ColMeasurements)
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

// ─── Checkups ─────────────────────────────────────────────────────────────────

// CheckupHandler manages pediatric visits.
type CheckupHandler struct{ db *repository.DB }

// NewCheckupHandler creates a new CheckupHandler.
func NewCheckupHandler(db *repository.DB) *CheckupHandler { return &CheckupHandler{db: db} }

// List godoc — GET /api/checkups
func (h *CheckupHandler) List(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	col := h.db.Collection(repository.ColCheckups)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, bson.M{"childId": childID},
		options.Find().SetSort(bson.D{{Key: "date", Value: -1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.Checkup
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.Checkup{}
	}
	c.JSON(http.StatusOK, items)
}

// Create godoc — POST /api/checkups
func (h *CheckupHandler) Create(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	var body models.Checkup
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.ChildID = childID
	body.CreatedAt = time.Now()
	if body.Prescriptions == nil {
		body.Prescriptions = []models.Prescription{}
	}

	col := h.db.Collection(repository.ColCheckups)
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

// ─── Milestones ───────────────────────────────────────────────────────────────

// MilestoneHandler manages developmental milestones.
type MilestoneHandler struct{ db *repository.DB }

// NewMilestoneHandler creates a new MilestoneHandler.
func NewMilestoneHandler(db *repository.DB) *MilestoneHandler { return &MilestoneHandler{db: db} }

// List godoc — GET /api/milestones
func (h *MilestoneHandler) List(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	col := h.db.Collection(repository.ColMilestones)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, bson.M{"childId": childID},
		options.Find().SetSort(bson.D{{Key: "date", Value: -1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.Milestone
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.Milestone{}
	}
	c.JSON(http.StatusOK, items)
}

// Create godoc — POST /api/milestones
func (h *MilestoneHandler) Create(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	var body models.Milestone
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.ChildID = childID
	body.CreatedAt = time.Now()
	if body.MediaURLs == nil {
		body.MediaURLs = []string{}
	}

	col := h.db.Collection(repository.ColMilestones)
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

// ─── Diary ────────────────────────────────────────────────────────────────────

// DiaryHandler manages daily diary entries.
type DiaryHandler struct{ db *repository.DB }

// NewDiaryHandler creates a new DiaryHandler.
func NewDiaryHandler(db *repository.DB) *DiaryHandler { return &DiaryHandler{db: db} }

// List godoc — GET /api/diary
func (h *DiaryHandler) List(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	col := h.db.Collection(repository.ColDiary)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, bson.M{"childId": childID},
		options.Find().SetSort(bson.D{{Key: "date", Value: -1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.DiaryEntry
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.DiaryEntry{}
	}
	c.JSON(http.StatusOK, items)
}

// Create godoc — POST /api/diary
func (h *DiaryHandler) Create(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	var body models.DiaryEntry
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.ChildID = childID
	body.CreatedAt = time.Now()
	if body.Data == nil {
		body.Data = map[string]interface{}{}
	}

	col := h.db.Collection(repository.ColDiary)
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

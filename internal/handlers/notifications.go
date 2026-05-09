package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"babyapp/backend/internal/middleware"
	"babyapp/backend/internal/models"
	"babyapp/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// NotificationHandler manages in-app notifications and auto-generates vaccine alerts.
type NotificationHandler struct{ db *repository.DB }

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(db *repository.DB) *NotificationHandler {
	return &NotificationHandler{db: db}
}

// List godoc — GET /api/notifications
// Auto-generates vaccine notifications for upcoming vaccines within 30 days.
func (h *NotificationHandler) List(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Auto-generate vaccine notifications
	if childID != "" {
		h.generateVaccineNotifications(ctx, userID, childID)
	}

	col := h.db.Collection(repository.ColNotifications)
	cursor, err := col.Find(ctx, bson.M{"userId": userID},
		options.Find().SetSort(bson.D{{Key: "date", Value: 1}}).SetLimit(50))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.AppNotification
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.AppNotification{}
	}
	c.JSON(http.StatusOK, items)
}

// generateVaccineNotifications creates notifications for pending vaccines within 30 days.
func (h *NotificationHandler) generateVaccineNotifications(ctx context.Context, userID, childID string) {
	vacCol := h.db.Collection(repository.ColVaccines)
	notifCol := h.db.Collection(repository.ColNotifications)
	today := time.Now()

	cursor, err := vacCol.Find(ctx, bson.M{
		"childId": childID,
		"status":  models.VaccinePending,
	})
	if err != nil {
		return
	}
	defer cursor.Close(ctx)

	var vaccines []models.Vaccine
	if err := cursor.All(ctx, &vaccines); err != nil {
		return
	}

	for _, v := range vaccines {
		scheduled, err := time.Parse("2006-01-02", v.ScheduledDate)
		if err != nil {
			continue
		}
		daysUntil := int(scheduled.Sub(today).Hours() / 24)
		if daysUntil < 0 || daysUntil > 30 {
			continue
		}

		// Skip if notification already exists
		count, _ := notifCol.CountDocuments(ctx, bson.M{
			"userId":    userID,
			"relatedId": v.ID.Hex(),
			"type":      models.NotifVaccine,
		})
		if count > 0 {
			continue
		}

		var urgency string
		switch {
		case daysUntil == 0:
			urgency = "¡HOY!"
		case daysUntil <= 7:
			urgency = fmt.Sprintf("en %d días", daysUntil)
		default:
			urgency = fmt.Sprintf("en %d días", daysUntil)
		}

		notif := models.AppNotification{
			UserID:    userID,
			ChildID:   childID,
			Type:      models.NotifVaccine,
			Title:     fmt.Sprintf("Vacuna: %s", v.Name),
			Message:   fmt.Sprintf("%s (%s) está programada %s.", v.Name, v.AgeLabel, urgency),
			Date:      v.ScheduledDate,
			Read:      false,
			RelatedID: v.ID.Hex(),
			CreatedAt: time.Now(),
		}
		_, _ = notifCol.InsertOne(ctx, notif)
	}
}

// MarkRead godoc — PATCH /api/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	col := h.db.Collection(repository.ColNotifications)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	_, err = col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"read": true}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// MarkAllRead godoc — PATCH /api/notifications/read-all
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)
	col := h.db.Collection(repository.ColNotifications)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	_, err := col.UpdateMany(ctx,
		bson.M{"userId": userID, "read": false},
		bson.M{"$set": bson.M{"read": true}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

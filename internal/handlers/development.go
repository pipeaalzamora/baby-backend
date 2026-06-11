package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"babyapp/backend/internal/healthdata"
	"babyapp/backend/internal/middleware"
	"babyapp/backend/internal/models"
	"babyapp/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type DevelopmentHandler struct {
	db *repository.DB
}

func NewDevelopmentHandler(db *repository.DB) *DevelopmentHandler {
	return &DevelopmentHandler{db: db}
}

// Advice godoc — GET /api/development/advice?ageMonths=9
func (h *DevelopmentHandler) Advice(c *gin.Context) {
	ageMonths, ok := parseAdviceAge(c.Query("ageMonths"))
	if !ok {
		childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
		if childID != "" {
			id, err := bson.ObjectIDFromHex(childID)
			if err == nil {
				var child models.Child
				if findErr := h.db.Collection(repository.ColChildren).FindOne(c.Request.Context(), bson.M{"_id": id}).Decode(&child); findErr == nil {
					if months, monthErr := monthsSinceBirth(child.BirthDate); monthErr == nil {
						ageMonths = months
						ok = true
					}
				}
			}
		}
	}
	if !ok {
		ageMonths = 6
	}

	c.JSON(http.StatusOK, healthdata.GetAgeAdvice(ageMonths))
}

func parseAdviceAge(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	age, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	if age < 0 {
		age = 0
	}
	if age > 60 {
		age = 60
	}
	return age, true
}

func monthsSinceBirth(birthDate string) (int, error) {
	birth, err := time.Parse("2006-01-02", strings.TrimSpace(birthDate))
	if err != nil {
		return 0, err
	}
	now := time.Now()
	months := (now.Year()-birth.Year())*12 + int(now.Month()-birth.Month())
	if now.Day() < birth.Day() {
		months--
	}
	if months < 0 {
		return 0, nil
	}
	return months, nil
}

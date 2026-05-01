package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"babyapp/backend/internal/middleware"
	"babyapp/backend/internal/models"
	"babyapp/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// PhotoHandler manages photo uploads and listing.
type PhotoHandler struct {
	db        *repository.DB
	uploadDir string
	baseURL   string
}

// NewPhotoHandler creates a new PhotoHandler.
func NewPhotoHandler(db *repository.DB, uploadDir, baseURL string) *PhotoHandler {
	return &PhotoHandler{db: db, uploadDir: uploadDir, baseURL: baseURL}
}

// List godoc — GET /api/photos
func (h *PhotoHandler) List(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	col := h.db.Collection(repository.ColPhotos)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, bson.M{"childId": childID},
		options.Find().SetSort(bson.D{{Key: "date", Value: -1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.Photo
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.Photo{}
	}
	c.JSON(http.StatusOK, items)
}

// Upload godoc — POST /api/photos (multipart/form-data)
func (h *PhotoHandler) Upload(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "archivo requerido"})
		return
	}
	defer file.Close()

	// Validate MIME type
	ext := filepath.Ext(header.Filename)
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}
	if !allowed[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "solo se permiten imágenes (jpg, png, webp, gif)"})
		return
	}

	// Ensure upload directory exists
	if err := os.MkdirAll(h.uploadDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al crear directorio"})
		return
	}

	filename := fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), randomHex(8), ext)
	dst := filepath.Join(h.uploadDir, filename)

	if err := c.SaveUploadedFile(header, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al guardar archivo"})
		return
	}

	url := fmt.Sprintf("%s/uploads/%s", h.baseURL, filename)

	// Parse tags from form
	var tags []string
	if t := c.PostForm("tags"); t != "" {
		if err := parseJSON(t, &tags); err != nil {
			tags = []string{}
		}
	}

	photo := models.Photo{
		ChildID:   childID,
		URL:       url,
		Date:      coalesce(c.PostForm("date"), time.Now().Format("2006-01-02")),
		Tags:      tags,
		Caption:   c.PostForm("caption"),
		CreatedAt: time.Now(),
	}

	col := h.db.Collection(repository.ColPhotos)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	res, err := col.InsertOne(ctx, photo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	photo.ID = res.InsertedID.(bson.ObjectID)
	c.JSON(http.StatusCreated, photo)
}

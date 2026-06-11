package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"babyapp/backend/internal/middleware"
	"babyapp/backend/internal/models"
	"babyapp/backend/internal/repository"
	"babyapp/backend/internal/storage"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// PhotoHandler manages photo uploads and listing.
type PhotoHandler struct {
	db        *repository.DB
	uploadDir string
	baseURL   string
	s3        *storage.S3Service
}

// NewPhotoHandler creates a new PhotoHandler.
func NewPhotoHandler(db *repository.DB, uploadDir, baseURL string, s3Service *storage.S3Service) *PhotoHandler {
	return &PhotoHandler{db: db, uploadDir: uploadDir, baseURL: baseURL, s3: s3Service}
}

type photoURLRequest struct {
	URL             string   `json:"url"`
	StorageProvider string   `json:"storageProvider"`
	Bucket          string   `json:"bucket"`
	Key             string   `json:"key"`
	ContentType     string   `json:"contentType"`
	SizeBytes       int64    `json:"sizeBytes"`
	Date            string   `json:"date"`
	Tags            []string `json:"tags"`
	Caption         string   `json:"caption"`
}

// Presign godoc — POST /api/photos/presign
func (h *PhotoHandler) Presign(c *gin.Context) {
	if !h.s3.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "S3 no configurado"})
		return
	}

	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
	if childID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perfil de bebé requerido"})
		return
	}

	var req presignMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	contentType, _, err := normalizeImageUpload(req.FileName, req.ContentType, req.SizeBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key, err := photoObjectKey(c.GetString(middleware.KeyUserID), childID, req.Date, req.FileName, contentType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	uploadURL, expiresAt, err := h.s3.PresignPut(ctx, key, contentType)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudo preparar la subida a S3"})
		return
	}

	c.JSON(http.StatusOK, presignMediaResponse{
		UploadURL:   uploadURL,
		Bucket:      h.s3.Bucket(),
		Key:         key,
		ContentType: contentType,
		ExpiresAt:   expiresAt,
		Headers:     map[string]string{"Content-Type": contentType},
	})
}

// List godoc — GET /api/photos
func (h *PhotoHandler) List(c *gin.Context) {
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
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
	for i := range items {
		h.signPhotoURL(ctx, &items[i])
	}
	c.JSON(http.StatusOK, items)
}

// Upload godoc — POST /api/photos (multipart/form-data or JSON with an existing URL)
func (h *PhotoHandler) Upload(c *gin.Context) {
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
	if childID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "perfil de bebé requerido"})
		return
	}

	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		var req photoURLRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "url requerida"})
			return
		}
		url := strings.TrimSpace(req.URL)
		if url == "" {
			if strings.ToLower(strings.TrimSpace(req.StorageProvider)) != "s3" || strings.TrimSpace(req.Key) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "url o key S3 requerida"})
				return
			}
		}

		userID := c.GetString(middleware.KeyUserID)
		photo := models.Photo{
			ChildID:         childID,
			URL:             url,
			StorageProvider: strings.ToLower(strings.TrimSpace(req.StorageProvider)),
			Bucket:          strings.TrimSpace(req.Bucket),
			Key:             strings.TrimSpace(req.Key),
			ContentType:     strings.TrimSpace(req.ContentType),
			SizeBytes:       req.SizeBytes,
			Date:            coalesce(req.Date, time.Now().Format("2006-01-02")),
			Tags:            req.Tags,
			Caption:         req.Caption,
			CreatedAt:       time.Now(),
		}
		if photo.StorageProvider == "s3" {
			if !h.s3.Enabled() {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "S3 no configurado"})
				return
			}
			if photo.Bucket == "" {
				photo.Bucket = h.s3.Bucket()
			}
			if photo.Bucket != h.s3.Bucket() {
				c.JSON(http.StatusBadRequest, gin.H{"error": "bucket S3 no permitido"})
				return
			}
			if !isS3ChildFolderKey(userID, childID, "photos", photo.Key) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "key S3 no permitida"})
				return
			}
		}
		if photo.Tags == nil {
			photo.Tags = []string{}
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
		h.signPhotoURL(ctx, &photo)
		c.JSON(http.StatusCreated, photo)
		return
	}

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

// Delete godoc — DELETE /api/photos/:id
func (h *PhotoHandler) Delete(c *gin.Context) {
	childID := resolveChildID(c, h.db, middleware.KeyChildID, middleware.KeyUserID)
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	col := h.db.Collection(repository.ColPhotos)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var photo models.Photo
	if err := col.FindOne(ctx, bson.M{"_id": id, "childId": childID}).Decode(&photo); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "foto no encontrada"})
		return
	}

	if strings.ToLower(photo.StorageProvider) == "s3" && photo.Key != "" {
		if err := h.s3.Delete(ctx, photo.Bucket, photo.Key); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudo eliminar la foto en S3"})
			return
		}
	}

	// Verificar ownership: la foto debe pertenecer al childId del usuario
	res, err := col.DeleteOne(ctx, bson.M{"_id": id, "childId": childID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if res.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "foto no encontrada"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *PhotoHandler) signPhotoURL(ctx context.Context, photo *models.Photo) {
	if photo == nil || strings.ToLower(photo.StorageProvider) != "s3" || photo.Key == "" || !h.s3.Enabled() {
		return
	}
	url, err := h.s3.PresignGet(ctx, photo.Bucket, photo.Key)
	if err == nil {
		photo.URL = url
	}
}

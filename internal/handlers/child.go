package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"babyapp/backend/internal/middleware"
	"babyapp/backend/internal/models"
	"babyapp/backend/internal/repository"
	"babyapp/backend/internal/storage"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ChildHandler manages the baby profile.
type ChildHandler struct {
	db *repository.DB
	s3 *storage.S3Service
}

// NewChildHandler creates a new ChildHandler.
func NewChildHandler(db *repository.DB, s3Service *storage.S3Service) *ChildHandler {
	return &ChildHandler{db: db, s3: s3Service}
}

type childPayload struct {
	Name          string   `json:"name"`
	BirthDate     string   `json:"birthDate"`
	Gender        string   `json:"gender"`
	BloodType     *string  `json:"bloodType"`
	PhotoURL      *string  `json:"photoUrl"`
	PhotoProvider *string  `json:"photoProvider"`
	PhotoBucket   *string  `json:"photoBucket"`
	PhotoKey      *string  `json:"photoKey"`
	PhotoMimeType *string  `json:"photoMimeType"`
	PhotoSize     *int64   `json:"photoSize"`
	BirthWeightKg *float64 `json:"birthWeightKg"`
	BirthHeightCm *float64 `json:"birthHeightCm"`
}

// Get godoc — GET /api/child
func (h *ChildHandler) Get(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	child, err := h.activeChild(ctx, userID, c.GetString(middleware.KeyChildID))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "perfil no encontrado"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	h.signChildURL(ctx, &child)
	c.JSON(http.StatusOK, child)
}

// List godoc — GET /api/children
func (h *ChildHandler) List(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)
	col := h.db.Collection(repository.ColChildren)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cur, err := col.Find(
		ctx,
		bson.M{"userId": userID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cur.Close(ctx)

	children := []models.Child{}
	if err := cur.All(ctx, &children); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range children {
		h.signChildURL(ctx, &children[i])
	}
	c.JSON(http.StatusOK, children)
}

// Create godoc — POST /api/children
func (h *ChildHandler) Create(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)

	var body childPayload
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	child, err := h.createChild(c.Request.Context(), userID, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	signCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	h.signChildURL(signCtx, &child)
	c.JSON(http.StatusOK, child)
}

// Upsert godoc — POST /api/child
func (h *ChildHandler) Upsert(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)

	var body childPayload
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	child, err := h.activeChild(ctx, userID, c.GetString(middleware.KeyChildID))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			created, createErr := h.createChild(c.Request.Context(), userID, body)
			if createErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": createErr.Error()})
				return
			}
			h.signChildURL(ctx, &created)
			c.JSON(http.StatusOK, created)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.updateChild(ctx, userID, child.ID.Hex(), body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.signChildURL(ctx, &updated)
	c.JSON(http.StatusOK, updated)
}

// Update godoc — PATCH /api/children/:id
func (h *ChildHandler) Update(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)
	childID := c.Param("id")

	var body childPayload
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	child, err := h.updateChild(ctx, userID, childID, body)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "perfil no encontrado"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	h.signChildURL(ctx, &child)
	c.JSON(http.StatusOK, child)
}

// Select godoc — POST /api/children/:id/select
func (h *ChildHandler) Select(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)
	childID := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	child, err := h.findOwnedChild(ctx, userID, childID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": "perfil no encontrado"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	if err := h.setActiveChild(ctx, userID, child.ID.Hex()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.signChildURL(ctx, &child)
	c.JSON(http.StatusOK, child)
}

// PresignPhoto godoc — POST /api/children/:id/photo/presign
func (h *ChildHandler) PresignPhoto(c *gin.Context) {
	if !h.s3.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "S3 no configurado"})
		return
	}

	userID := c.GetString(middleware.KeyUserID)
	childID := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if _, err := h.findOwnedChild(ctx, userID, childID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "perfil no encontrado"})
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

	key, err := childProfileObjectKey(userID, childID, req.FileName, contentType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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

func (h *ChildHandler) createChild(ctx context.Context, userID string, body childPayload) (models.Child, error) {
	if err := validateChildPayload(body); err != nil {
		return models.Child{}, err
	}

	now := time.Now()
	child := models.Child{
		UserID:        userID,
		Name:          strings.TrimSpace(body.Name),
		BirthDate:     strings.TrimSpace(body.BirthDate),
		Gender:        strings.ToUpper(strings.TrimSpace(body.Gender)),
		BirthWeightKg: optionalFloat(body.BirthWeightKg),
		BirthHeightCm: optionalFloat(body.BirthHeightCm),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if body.BloodType != nil {
		child.BloodType = strings.TrimSpace(*body.BloodType)
	}
	if body.PhotoURL != nil {
		child.PhotoURL = strings.TrimSpace(*body.PhotoURL)
	}
	if body.PhotoKey != nil {
		if strings.TrimSpace(*body.PhotoKey) != "" {
			return models.Child{}, errors.New("crea el perfil antes de subir la foto S3")
		}
	}

	col := h.db.Collection(repository.ColChildren)
	res, err := col.InsertOne(ctx, child)
	if err != nil {
		return models.Child{}, err
	}
	child.ID = res.InsertedID.(bson.ObjectID)

	if err := h.setActiveChild(ctx, userID, child.ID.Hex()); err != nil {
		return models.Child{}, err
	}
	return child, nil
}

func (h *ChildHandler) updateChild(ctx context.Context, userID, childID string, body childPayload) (models.Child, error) {
	if err := validateChildPayload(body); err != nil {
		return models.Child{}, err
	}

	id, err := bson.ObjectIDFromHex(childID)
	if err != nil {
		return models.Child{}, errors.New("id de perfil inválido")
	}

	setFields := bson.M{
		"name":          strings.TrimSpace(body.Name),
		"birthDate":     strings.TrimSpace(body.BirthDate),
		"gender":        strings.ToUpper(strings.TrimSpace(body.Gender)),
		"birthWeightKg": optionalFloat(body.BirthWeightKg),
		"birthHeightCm": optionalFloat(body.BirthHeightCm),
		"updatedAt":     time.Now(),
	}
	unsetFields := bson.M{"modelKey": ""}
	if body.BloodType != nil {
		if bloodType := strings.TrimSpace(*body.BloodType); bloodType != "" {
			setFields["bloodType"] = bloodType
		} else {
			unsetFields["bloodType"] = ""
		}
	}
	if body.PhotoURL != nil {
		if photoURL := strings.TrimSpace(*body.PhotoURL); photoURL != "" {
			setFields["photoUrl"] = photoURL
			unsetFields["photoProvider"] = ""
			unsetFields["photoBucket"] = ""
			unsetFields["photoKey"] = ""
			unsetFields["photoMimeType"] = ""
			unsetFields["photoSize"] = ""
		} else {
			unsetFields["photoUrl"] = ""
		}
	}
	if body.PhotoKey != nil {
		photoKey := strings.TrimSpace(*body.PhotoKey)
		if photoKey != "" {
			photoProvider := strings.ToLower(coalescePointer(body.PhotoProvider, "s3"))
			if photoProvider != "s3" {
				return models.Child{}, errors.New("proveedor de foto no permitido")
			}
			if !h.s3.Enabled() {
				return models.Child{}, errors.New("S3 no configurado")
			}
			photoBucket := coalescePointer(body.PhotoBucket, h.s3.Bucket())
			if photoBucket != h.s3.Bucket() {
				return models.Child{}, errors.New("bucket S3 no permitido")
			}
			if !isS3ChildFolderKey(userID, childID, "profile", photoKey) {
				return models.Child{}, errors.New("key S3 no permitida")
			}
			setFields["photoProvider"] = photoProvider
			setFields["photoBucket"] = photoBucket
			setFields["photoKey"] = photoKey
			setFields["photoMimeType"] = coalescePointer(body.PhotoMimeType, "")
			setFields["photoSize"] = optionalInt64(body.PhotoSize)
			unsetFields["photoUrl"] = ""
		} else {
			unsetFields["photoProvider"] = ""
			unsetFields["photoBucket"] = ""
			unsetFields["photoKey"] = ""
			unsetFields["photoMimeType"] = ""
			unsetFields["photoSize"] = ""
		}
	}

	update := bson.M{"$set": setFields}
	if len(unsetFields) > 0 {
		update["$unset"] = unsetFields
	}

	col := h.db.Collection(repository.ColChildren)
	res, err := col.UpdateOne(ctx, bson.M{"_id": id, "userId": userID}, update)
	if err != nil {
		return models.Child{}, err
	}
	if res.MatchedCount == 0 {
		return models.Child{}, mongo.ErrNoDocuments
	}

	return h.findOwnedChild(ctx, userID, childID)
}

func (h *ChildHandler) activeChild(ctx context.Context, userID, activeChildID string) (models.Child, error) {
	if activeChildID != "" {
		child, err := h.findOwnedChild(ctx, userID, activeChildID)
		if err == nil {
			return child, nil
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return models.Child{}, err
		}
	}

	child, err := h.firstChild(ctx, userID)
	if err != nil {
		return models.Child{}, err
	}
	_ = h.setActiveChild(ctx, userID, child.ID.Hex())
	return child, nil
}

func (h *ChildHandler) firstChild(ctx context.Context, userID string) (models.Child, error) {
	col := h.db.Collection(repository.ColChildren)
	var child models.Child
	err := col.FindOne(
		ctx,
		bson.M{"userId": userID},
		options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: 1}}),
	).Decode(&child)
	return child, err
}

func (h *ChildHandler) findOwnedChild(ctx context.Context, userID, childID string) (models.Child, error) {
	id, err := bson.ObjectIDFromHex(childID)
	if err != nil {
		return models.Child{}, errors.New("id de perfil inválido")
	}

	col := h.db.Collection(repository.ColChildren)
	var child models.Child
	err = col.FindOne(ctx, bson.M{"_id": id, "userId": userID}).Decode(&child)
	return child, err
}

func (h *ChildHandler) setActiveChild(ctx context.Context, userID, childID string) error {
	userObjID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil
	}
	_, err = h.db.Collection(repository.ColUsers).UpdateOne(
		ctx,
		bson.M{"_id": userObjID},
		bson.M{"$set": bson.M{"childId": childID}},
	)
	return err
}

func validateChildPayload(body childPayload) error {
	if strings.TrimSpace(body.Name) == "" {
		return errors.New("nombre requerido")
	}

	birthDate := strings.TrimSpace(body.BirthDate)
	if birthDate == "" {
		return errors.New("fecha de nacimiento requerida")
	}
	parsed, err := time.Parse("2006-01-02", birthDate)
	if err != nil {
		return errors.New("fecha de nacimiento inválida")
	}
	if parsed.After(time.Now()) {
		return errors.New("fecha de nacimiento no puede ser futura")
	}

	gender := strings.ToUpper(strings.TrimSpace(body.Gender))
	if gender != "M" && gender != "F" {
		return errors.New("sexo inválido")
	}

	if body.BirthWeightKg != nil && *body.BirthWeightKg < 0 {
		return errors.New("peso al nacer inválido")
	}
	if body.BirthHeightCm != nil && *body.BirthHeightCm < 0 {
		return errors.New("talla al nacer inválida")
	}
	return nil
}

func optionalFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func optionalInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func coalescePointer(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	if trimmed := strings.TrimSpace(*value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func (h *ChildHandler) signChildURL(ctx context.Context, child *models.Child) {
	if child == nil || strings.ToLower(child.PhotoProvider) != "s3" || child.PhotoKey == "" || !h.s3.Enabled() {
		return
	}
	url, err := h.s3.PresignGet(ctx, child.PhotoBucket, child.PhotoKey)
	if err == nil {
		child.PhotoURL = url
	}
}

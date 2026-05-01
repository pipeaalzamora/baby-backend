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

// RecipeHandler manages baby food recipes and food introductions.
type RecipeHandler struct{ db *repository.DB }

// NewRecipeHandler creates a new RecipeHandler.
func NewRecipeHandler(db *repository.DB) *RecipeHandler { return &RecipeHandler{db: db} }

// List godoc — GET /api/recipes?stage=6m
func (h *RecipeHandler) List(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	filter := bson.M{"childId": childID}
	if stage := c.Query("stage"); stage != "" {
		filter["stage"] = stage
	}

	col := h.db.Collection(repository.ColRecipes)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "stage", Value: 1}, {Key: "name", Value: 1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.Recipe
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.Recipe{}
	}
	c.JSON(http.StatusOK, items)
}

// Create godoc — POST /api/recipes
func (h *RecipeHandler) Create(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	var body models.Recipe
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.ChildID = childID
	body.CreatedAt = time.Now()

	col := h.db.Collection(repository.ColRecipes)
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

// ToggleFavorite godoc — PATCH /api/recipes/:id/favorite
func (h *RecipeHandler) ToggleFavorite(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var body struct {
		IsFavorite bool `json:"isFavorite"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	col := h.db.Collection(repository.ColRecipes)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var recipe models.Recipe
	if err := col.FindOneAndUpdate(ctx, bson.M{"_id": id},
		bson.M{"$set": bson.M{"isFavorite": body.IsFavorite}}, opts).Decode(&recipe); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "receta no encontrada"})
		return
	}
	c.JSON(http.StatusOK, recipe)
}

// ListIntroductions godoc — GET /api/recipes/introductions
func (h *RecipeHandler) ListIntroductions(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	col := h.db.Collection(repository.ColFoodIntroductions)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := col.Find(ctx, bson.M{"childId": childID},
		options.Find().SetSort(bson.D{{Key: "date", Value: -1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var items []models.FoodIntroduction
	if err := cursor.All(ctx, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []models.FoodIntroduction{}
	}
	c.JSON(http.StatusOK, items)
}

// CreateIntroduction godoc — POST /api/recipes/introductions
func (h *RecipeHandler) CreateIntroduction(c *gin.Context) {
	childID := c.GetString(middleware.KeyChildID)
	var body models.FoodIntroduction
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.ChildID = childID
	body.CreatedAt = time.Now()

	col := h.db.Collection(repository.ColFoodIntroductions)
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

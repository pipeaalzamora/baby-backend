package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"babyapp/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// randomHex generates a cryptographically random hex string of n bytes.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// parseJSON unmarshals a JSON string into v.
func parseJSON(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}

// coalesce returns the first non-empty string.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// resolveChildID devuelve el childID del contexto Gin.
// Si está vacío (token recién creado sin child aún), lo busca en la colección
// children por userId. Esto ocurre cuando el usuario acaba de registrarse y
// aún no tiene childId en el JWT.
func resolveChildID(c *gin.Context, db *repository.DB, childIDKey, userIDKey string) string {
	childID := c.GetString(childIDKey)
	if childID != "" {
		return childID
	}

	userID := c.GetString(userIDKey)
	if userID == "" {
		return ""
	}

	col := db.Collection(repository.ColChildren)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var child struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := col.FindOne(ctx, bson.M{"userId": userID}).Decode(&child); err == nil {
		return child.ID.Hex()
	}
	return ""
}

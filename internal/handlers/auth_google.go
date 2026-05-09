package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"babyapp/backend/internal/middleware"
	"babyapp/backend/internal/models"
	"babyapp/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// GoogleAuthHandler handles Google OAuth login via ID token verification.
type GoogleAuthHandler struct {
	db             *repository.DB
	jwtSecret      string
	jwtExpiry      time.Duration
	googleClientID string
}

// NewGoogleAuthHandler creates a new GoogleAuthHandler.
func NewGoogleAuthHandler(db *repository.DB, secret string, expiry time.Duration, clientID string) *GoogleAuthHandler {
	return &GoogleAuthHandler{
		db:             db,
		jwtSecret:      secret,
		jwtExpiry:      expiry,
		googleClientID: clientID,
	}
}

type googleLoginRequest struct {
	Credential string `json:"credential" binding:"required"`
}

// googleTokenInfo holds the fields we care about from Google's tokeninfo endpoint.
type googleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Aud           string `json:"aud"`
}

// verifyGoogleToken calls Google's tokeninfo endpoint to validate the ID token.
// This is the simplest approach that doesn't require additional libraries.
func (h *GoogleAuthHandler) verifyGoogleToken(ctx context.Context, idToken string) (*googleTokenInfo, error) {
	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", idToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("crear request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llamar tokeninfo: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("leer respuesta: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token inválido (status %d)", resp.StatusCode)
	}

	var info googleTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parsear respuesta: %w", err)
	}

	// Validate audience matches our client ID
	if h.googleClientID != "" && info.Aud != h.googleClientID {
		return nil, fmt.Errorf("audience no coincide: got %s", info.Aud)
	}

	if info.Email == "" || info.Sub == "" {
		return nil, fmt.Errorf("token sin email o sub")
	}

	return &info, nil
}

// GoogleLogin godoc — POST /api/auth/google
// Receives a Google ID token (credential from GIS), verifies it,
// creates or finds the user, and returns our own JWT.
func (h *GoogleAuthHandler) GoogleLogin(c *gin.Context) {
	var req googleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "credential requerido"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	// Verify the Google ID token
	info, err := h.verifyGoogleToken(ctx, req.Credential)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token de Google inválido: " + err.Error()})
		return
	}

	col := h.db.Collection(repository.ColUsers)

	// Find or create user by email
	var user models.User
	err = col.FindOne(ctx, bson.M{"email": info.Email}).Decode(&user)

	if err == mongo.ErrNoDocuments {
		// New user — create account
		newUser := models.User{
			Email:     info.Email,
			Name:      info.Name,
			CreatedAt: time.Now(),
		}
		res, insertErr := col.InsertOne(ctx, newUser)
		if insertErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error al crear usuario"})
			return
		}
		newUser.ID = res.InsertedID.(bson.ObjectID)
		user = newUser
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno"})
		return
	}

	// Issue our own JWT
	token, err := middleware.SignToken(user.ID.Hex(), user.ChildID, h.jwtSecret, h.jwtExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error al generar token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":      user.ID.Hex(),
			"email":   user.Email,
			"name":    user.Name,
			"childId": user.ChildID,
			"picture": info.Picture,
		},
	})
}

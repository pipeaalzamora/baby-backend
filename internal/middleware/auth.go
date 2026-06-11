// Package middleware provides Gin middleware for authentication and CORS.
package middleware

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"babyapp/backend/internal/models"
	"babyapp/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	KeyUserID      = "userID"
	KeyChildID     = "childID"
	KeyFirebaseUID = "firebaseUID"

	firebaseCertsURL = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"
)

// FirebaseToken is the verified subset of Firebase Auth ID token claims used by the app.
type FirebaseToken struct {
	UID     string
	Email   string
	Name    string
	Picture string
}

// FirebaseVerifier verifies Firebase Auth ID tokens using Google's public certs.
type FirebaseVerifier struct {
	projectID  string
	issuer     string
	httpClient *http.Client

	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
}

type firebaseClaims struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	jwt.RegisteredClaims
}

// NewFirebaseVerifier creates a token verifier for one Firebase project.
func NewFirebaseVerifier(projectID string) *FirebaseVerifier {
	return &FirebaseVerifier{
		projectID:  projectID,
		issuer:     "https://securetoken.google.com/" + projectID,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		keys:       map[string]*rsa.PublicKey{},
	}
}

// VerifyIDToken validates signature, issuer, audience and expiry.
func (v *FirebaseVerifier) VerifyIDToken(ctx context.Context, idToken string) (*FirebaseToken, error) {
	claims := &firebaseClaims{}
	token, err := jwt.ParseWithClaims(
		idToken,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				return nil, fmt.Errorf("algoritmo inesperado: %s", token.Method.Alg())
			}

			kid, ok := token.Header["kid"].(string)
			if !ok || kid == "" {
				return nil, fmt.Errorf("kid requerido")
			}
			return v.publicKey(ctx, kid)
		},
		jwt.WithAudience(v.projectID),
		jwt.WithIssuer(v.issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
	)
	if err != nil {
		return nil, err
	}
	if !token.Valid || claims.Subject == "" {
		return nil, fmt.Errorf("token inválido")
	}

	return &FirebaseToken{
		UID:     claims.Subject,
		Email:   strings.TrimSpace(claims.Email),
		Name:    strings.TrimSpace(claims.Name),
		Picture: strings.TrimSpace(claims.Picture),
	}, nil
}

func (v *FirebaseVerifier) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	cacheValid := time.Now().Before(v.expires)
	v.mu.RUnlock()
	if ok && cacheValid {
		return key, nil
	}

	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok = v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("llave Firebase no encontrada")
	}
	return key, nil
}

func (v *FirebaseVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, firebaseCertsURL, nil)
	if err != nil {
		return err
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("certificados Firebase status %d", resp.StatusCode)
	}

	var raw map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey, len(raw))
	for kid, certPEM := range raw {
		block, _ := pem.Decode([]byte(certPEM))
		if block == nil {
			return fmt.Errorf("certificado Firebase inválido")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return err
		}
		publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("certificado Firebase no es RSA")
		}
		keys[kid] = publicKey
	}

	v.mu.Lock()
	v.keys = keys
	v.expires = time.Now().Add(cacheMaxAge(resp.Header.Get("Cache-Control")))
	v.mu.Unlock()
	return nil
}

func cacheMaxAge(cacheControl string) time.Duration {
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "max-age=") {
			continue
		}
		seconds, err := strconv.Atoi(strings.TrimPrefix(part, "max-age="))
		if err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return time.Hour
}

// RequireFirebaseAuth validates Firebase ID tokens and resolves the local user.
func RequireFirebaseAuth(db *repository.DB, verifier *FirebaseVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no autorizado"})
			return
		}

		idToken := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if idToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token requerido"})
			return
		}

		token, err := verifier.VerifyIDToken(c.Request.Context(), idToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token de Firebase inválido"})
			return
		}

		user, err := upsertFirebaseUser(c.Request.Context(), db, token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "error al resolver usuario"})
			return
		}

		c.Set(KeyUserID, user.ID.Hex())
		c.Set(KeyChildID, user.ChildID)
		c.Set(KeyFirebaseUID, token.UID)
		c.Next()
	}
}

func upsertFirebaseUser(ctx context.Context, db *repository.DB, token *FirebaseToken) (*models.User, error) {
	email := token.Email
	name := token.Name
	picture := token.Picture
	if name == "" {
		name = fallbackName(email, token.UID)
	}

	col := db.Collection(repository.ColUsers)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var user models.User
	err := col.FindOne(ctx, bson.M{"firebaseUid": token.UID}).Decode(&user)
	if err == nil {
		update := bson.M{}
		if email != "" && email != user.Email {
			update["email"] = email
			user.Email = email
		}
		if name != "" && name != user.Name {
			update["name"] = name
			user.Name = name
		}
		if picture != "" && picture != user.Picture {
			update["picture"] = picture
			user.Picture = picture
		}
		if len(update) > 0 {
			if _, updateErr := col.UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": update}); updateErr != nil {
				return nil, updateErr
			}
		}
		return &user, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}

	if email != "" {
		err = col.FindOne(ctx, bson.M{"email": email}).Decode(&user)
		if err == nil {
			update := bson.M{
				"firebaseUid": token.UID,
				"name":        name,
			}
			if picture != "" {
				update["picture"] = picture
			}
			if _, updateErr := col.UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": update}); updateErr != nil {
				return nil, updateErr
			}
			user.FirebaseUID = token.UID
			user.Name = name
			user.Picture = picture
			return &user, nil
		}
		if err != mongo.ErrNoDocuments {
			return nil, err
		}
	}

	newUser := models.User{
		FirebaseUID: token.UID,
		Email:       email,
		Name:        name,
		Picture:     picture,
		CreatedAt:   time.Now(),
	}
	res, err := col.InsertOne(ctx, newUser)
	if err != nil {
		return nil, err
	}
	newUser.ID = res.InsertedID.(bson.ObjectID)
	return &newUser, nil
}

func fallbackName(email, uid string) string {
	if email != "" {
		if idx := strings.Index(email, "@"); idx > 0 {
			return email[:idx]
		}
		return email
	}
	return uid
}

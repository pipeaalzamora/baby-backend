// Package middleware provides Gin middleware for authentication and CORS.
package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims holds the JWT payload.
type Claims struct {
	UserID  string `json:"sub"`
	ChildID string `json:"childId,omitempty"`
	jwt.RegisteredClaims
}

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey string

const (
	KeyUserID  = "userID"
	KeyChildID = "childID"
)

// SignToken creates a signed JWT for the given user.
func SignToken(userID, childID, secret string, expiry time.Duration) (string, error) {
	claims := Claims{
		UserID:  userID,
		ChildID: childID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// RequireAuth returns a Gin middleware that validates Bearer JWT tokens.
// On success it sets userID and childID in the Gin context.
func RequireAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no autorizado"})
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("método de firma inesperado: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token inválido o expirado"})
			return
		}

		c.Set(KeyUserID, claims.UserID)
		c.Set(KeyChildID, claims.ChildID)
		c.Next()
	}
}

package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
)

const (
	// ContextKeyUserID is the key for storing user ID in gin.Context
	ContextKeyUserID = "user_id"
	// ContextKeyUserEmail is the key for storing user email in gin.Context
	ContextKeyUserEmail = "user_email"
	// ContextKeyRequiredFeature is the key for storing required feature in gin.Context
	ContextKeyRequiredFeature = "required_feature"
)

// JWTAuthMiddleware creates a JWT authentication middleware
// It extracts the token from Authorization header, validates it, and stores user info in context
// If feature is provided, it will check if the user has the required feature
func JWTAuthMiddleware(app contracts.App, feature ...string) gin.HandlerFunc {
	requiredFeature := ""
	if len(feature) > 0 && feature[0] != "" {
		requiredFeature = feature[0]
	}

	return func(c *gin.Context) {
		// Get JWT service from DI container
		var jwtService auroraFeature.JWTService
		if err := app.Find(&jwtService); err != nil {
			c.JSON(500, gin.H{
				"message": "JWT service not available",
			})
			c.Abort()
			return
		}

		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{
				"message": "Authorization header is required",
			})
			c.Abort()
			return
		}

		// Check if it's a Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(401, gin.H{
				"message": "Invalid authorization header format. Expected: Bearer <token>",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(401, gin.H{
				"message": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Check feature permission if required
		if requiredFeature != "" {
			hasFeature := false
			for _, f := range claims.Features {
				if f == requiredFeature {
					hasFeature = true
					break
				}
			}
			if !hasFeature {
				// Return 403 Forbidden when user is authenticated but lacks required feature
				c.JSON(403, gin.H{
					"message": "Insufficient permissions",
				})
				c.Abort()
				return
			}
		}

		// Store user information in context for use in handlers
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUserEmail, claims.Email)
		if requiredFeature != "" {
			c.Set(ContextKeyRequiredFeature, requiredFeature)
		}

		// Continue to next handler
		c.Next()
	}
}

// GetUserID extracts user ID from gin.Context (set by JWTAuthMiddleware)
func GetUserID(c *gin.Context) (int64, bool) {
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		return 0, false
	}

	id, ok := userID.(int64)
	return id, ok
}

// GetUserEmail extracts user email from gin.Context (set by JWTAuthMiddleware)
func GetUserEmail(c *gin.Context) (string, bool) {
	email, exists := c.Get(ContextKeyUserEmail)
	if !exists {
		return "", false
	}

	emailStr, ok := email.(string)
	return emailStr, ok
}

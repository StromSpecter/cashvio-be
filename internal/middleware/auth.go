package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		tokenStr := parts[1]
		claims, err := util.ParseJWT(tokenStr, secret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims["user_id"])
		c.Set("email", claims["email"])
		c.Set("role", claims["role"])
		c.Next()
	}
}

// RequirePremium restricts a route to users whose current role (read from DB,
// not the JWT claim so recently-activated premium is honored) is premium and
// not expired.
func RequirePremium(getUser func(ctx context.Context, id uuid.UUID) (*model.User, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr, ok := c.Get("user_id")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		id, err := uuid.Parse(idStr.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
			c.Abort()
			return
		}

		user, err := getUser(c.Request.Context(), id)
		if err != nil || user == nil || !user.IsPremium() {
			c.JSON(http.StatusForbidden, gin.H{"error": "premium subscription required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

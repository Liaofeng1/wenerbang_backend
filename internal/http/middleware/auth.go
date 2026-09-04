package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	jwtauth "wenbang/internal/auth"
	"wenbang/internal/http/httpx"
)

const ContextUserIDKey = "userID"
const ContextUsernameKey = "username"

func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			httpx.Fail(c, http.StatusUnauthorized, "未登录")
			c.Abort()
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		claims, err := jwtauth.ParseToken(token, secret)
		if err != nil {
			httpx.Fail(c, http.StatusUnauthorized, "登录已失效")
			c.Abort()
			return
		}
		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextUsernameKey, claims.Username)
		c.Next()
	}
}

func UserID(c *gin.Context) uint {
	v, _ := c.Get(ContextUserIDKey)
	id, _ := v.(uint)
	return id
}

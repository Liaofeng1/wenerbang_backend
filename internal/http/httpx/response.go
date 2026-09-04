package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func Fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}

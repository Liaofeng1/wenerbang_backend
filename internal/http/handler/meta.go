package handler

import (
	"github.com/gin-gonic/gin"

	"wenbang/internal/http/httpx"
	"wenbang/internal/model"
)

func DegreeTags(c *gin.Context) {
	httpx.OK(c, model.DegreeTags)
}

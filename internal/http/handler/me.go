package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"wenbang/internal/http/httpx"
	"wenbang/internal/http/middleware"
	"wenbang/internal/service"
)

type MeHandler struct {
	auth *service.AuthService
}

func NewMeHandler(auth *service.AuthService) *MeHandler {
	return &MeHandler{auth: auth}
}

func (h *MeHandler) Me(c *gin.Context) {
	user, err := h.auth.GetUser(middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, http.StatusUnauthorized, "用户不存在")
		return
	}
	httpx.OK(c, user)
}

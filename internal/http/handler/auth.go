package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"wenbang/internal/http/httpx"
	"wenbang/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type registerReq struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Nickname   string `json:"nickname"`
	School     string `json:"school"`
	InviteCode string `json:"invite_code"`
	Gender     string `json:"gender"`
	Region     string `json:"region"`
	CityTier   string `json:"city_tier"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	res, err := h.auth.Register(service.RegisterInput{
		Username:   req.Username,
		Password:   req.Password,
		Nickname:   req.Nickname,
		School:     req.School,
		InviteCode: req.InviteCode,
		Gender:     req.Gender,
		Region:     req.Region,
		CityTier:   req.CityTier,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUsernameTaken):
			httpx.Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrWeakInput),
			errors.Is(err, service.ErrInvalidInviteCode),
			errors.Is(err, service.ErrInvalidProfile):
			httpx.Fail(c, http.StatusBadRequest, err.Error())
		default:
			httpx.Fail(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	httpx.OK(c, res)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	res, err := h.auth.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			httpx.Fail(c, http.StatusUnauthorized, err.Error())
			return
		}
		httpx.Fail(c, http.StatusInternalServerError, "登录失败")
		return
	}
	httpx.OK(c, res)
}

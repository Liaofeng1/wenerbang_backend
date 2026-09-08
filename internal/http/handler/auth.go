package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"wenbang/internal/http/httpx"
	"wenbang/internal/service"
)

type AuthHandler struct {
	auth  *service.AuthService
	email *service.EmailService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth, email: service.NewEmailService(auth.DB())}
}

type registerReq struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Email      string `json:"email"`
	Nickname   string `json:"nickname"`
	School     string `json:"school"`
	Major      string `json:"major"`
	Gender     string `json:"gender"`
	Region     string `json:"region"`
	CityTier   string `json:"city_tier"`
	InviteCode string `json:"invite_code"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type sendCodeReq struct {
	Email string `json:"email"`
}

type verifyCodeReq struct {
	Email string `json:"email"`
	Code  string `json:"code"`
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
		Email:      req.Email,
		Nickname:   req.Nickname,
		School:     req.School,
		Major:      req.Major,
		Gender:     req.Gender,
		Region:     req.Region,
		CityTier:   req.CityTier,
		InviteCode: req.InviteCode,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUsernameTaken):
			httpx.Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrEmailTaken):
			httpx.Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrEmailNotVerified):
			httpx.Fail(c, http.StatusBadRequest, err.Error())
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

func (h *AuthHandler) SendEmailCode(c *gin.Context) {
	var req sendCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := h.email.SendVerificationCode(req.Email); err != nil {
		switch {
		case errors.Is(err, service.ErrEmailTaken):
			httpx.Fail(c, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrEmailCodeCooldown):
			httpx.Fail(c, http.StatusTooManyRequests, err.Error())
		default:
			httpx.Fail(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	httpx.OK(c, gin.H{"message": "验证码已发送"})
}

func (h *AuthHandler) VerifyEmailCode(c *gin.Context) {
	var req verifyCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := h.email.VerifyCode(req.Email, req.Code); err != nil {
		switch {
		case errors.Is(err, service.ErrEmailCodeInvalid):
			httpx.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrEmailVerified):
			httpx.Fail(c, http.StatusBadRequest, err.Error())
		default:
			httpx.Fail(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	httpx.OK(c, gin.H{"message": "邮箱验证成功"})
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

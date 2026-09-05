package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"wenbang/internal/http/httpx"
	"wenbang/internal/http/middleware"
	"wenbang/internal/service"
)

type SurveyHandler struct {
	surveys *service.SurveyService
}

func NewSurveyHandler(surveys *service.SurveyService) *SurveyHandler {
	return &SurveyHandler{surveys: surveys}
}

func (h *SurveyHandler) Create(c *gin.Context) {
	var req service.CreateSurveyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	survey, err := h.surveys.Create(middleware.UserID(c), req)
	if err != nil {
		mapSurveyErr(c, err)
		return
	}
	httpx.OK(c, survey)
}

func (h *SurveyHandler) List(c *gin.Context) {
	list, err := h.surveys.ListOpen(middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "获取列表失败")
		return
	}
	httpx.OK(c, list)
}

func (h *SurveyHandler) ListMine(c *gin.Context) {
	list, err := h.surveys.ListMine(middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "获取列表失败")
		return
	}
	httpx.OK(c, list)
}

func (h *SurveyHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	survey, err := h.surveys.Get(id)
	if err != nil {
		mapSurveyErr(c, err)
		return
	}
	httpx.OK(c, survey)
}

func (h *SurveyHandler) Start(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	view, err := h.surveys.Start(id, middleware.UserID(c))
	if err != nil {
		mapSurveyErr(c, err)
		return
	}
	httpx.OK(c, view)
}

func (h *SurveyHandler) Leave(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	view, err := h.surveys.Leave(id, middleware.UserID(c))
	if err != nil {
		mapSurveyErr(c, err)
		return
	}
	httpx.OK(c, view)
}

func (h *SurveyHandler) Return(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	view, err := h.surveys.Return(id, middleware.UserID(c))
	if err != nil {
		mapSurveyErr(c, err)
		return
	}
	httpx.OK(c, view)
}

func (h *SurveyHandler) Session(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	view, err := h.surveys.GetSession(id, middleware.UserID(c))
	if err != nil {
		mapSurveyErr(c, err)
		return
	}
	httpx.OK(c, view)
}

func (h *SurveyHandler) Complete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	completion, err := h.surveys.Complete(id, middleware.UserID(c))
	if err != nil {
		mapSurveyErr(c, err)
		return
	}
	httpx.OK(c, completion)
}

func (h *SurveyHandler) ListMyCompletions(c *gin.Context) {
	list, err := h.surveys.ListMyCompletions(middleware.UserID(c))
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, "获取列表失败")
		return
	}
	httpx.OK(c, list)
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, "无效 ID")
		return 0, false
	}
	return uint(id), true
}

func mapSurveyErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		httpx.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrInsufficientPts),
		errors.Is(err, service.ErrAlreadyCompleted),
		errors.Is(err, service.ErrOwnSurvey),
		errors.Is(err, service.ErrSurveyClosed),
		errors.Is(err, service.ErrBadSurveyInput),
		errors.Is(err, service.ErrNeedOpenFirst),
		errors.Is(err, service.ErrAwayTooShort),
		errors.Is(err, service.ErrDegreeMismatch):
		httpx.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrForbidden):
		httpx.Fail(c, http.StatusForbidden, err.Error())
	default:
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
	}
}

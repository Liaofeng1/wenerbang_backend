package handler

import (
	"github.com/gin-gonic/gin"

	"wenbang/internal/http/httpx"
	"wenbang/internal/model"
)

func ProfileOptions(c *gin.Context) {
	httpx.OK(c, gin.H{
		"genders":    model.Genders,
		"regions":    model.Regions,
		"city_tiers": model.CityTiers,
		"majors": []string{
			"哲学", "经济学", "法学", "教育学", "文学", "历史学",
			"理学", "工学", "农学", "医学", "军事学", "管理学", "艺术学",
		},
	})
}

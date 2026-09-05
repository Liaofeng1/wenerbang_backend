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
	})
}

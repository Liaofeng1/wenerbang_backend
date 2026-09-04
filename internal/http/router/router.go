package router

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"wenbang/internal/config"
	"wenbang/internal/http/handler"
	"wenbang/internal/http/middleware"
	"wenbang/internal/service"
)

func New(db *gorm.DB) *gin.Engine {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authSvc := service.NewAuthService(db)
	surveySvc := service.NewSurveyService(db)
	authH := handler.NewAuthHandler(authSvc)
	surveyH := handler.NewSurveyHandler(surveySvc)
	meH := handler.NewMeHandler(authSvc)

	api := r.Group("/api/v1")
	{
		api.POST("/auth/register", authH.Register)
		api.POST("/auth/login", authH.Login)

		auth := api.Group("")
		auth.Use(middleware.JWTAuth(config.JWTSecret()))
		{
			auth.GET("/me", meH.Me)
			auth.POST("/surveys", surveyH.Create)
			auth.GET("/surveys", surveyH.List)
			auth.GET("/surveys/mine", surveyH.ListMine)
			auth.GET("/surveys/:id", surveyH.Get)
			auth.POST("/surveys/:id/start", surveyH.Start)
			auth.POST("/surveys/:id/leave", surveyH.Leave)
			auth.POST("/surveys/:id/return", surveyH.Return)
			auth.GET("/surveys/:id/session", surveyH.Session)
			auth.POST("/surveys/:id/complete", surveyH.Complete)
			auth.GET("/completions/mine", surveyH.ListMyCompletions)
		}
	}

	return r
}

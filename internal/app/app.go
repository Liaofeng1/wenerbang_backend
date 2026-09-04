package app

import (
	"fmt"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wenbang/internal/config"
	"wenbang/internal/http/router"
	"wenbang/internal/model"
)

func Run() error {
	dsn := config.DatabaseDSN()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(&model.User{}, &model.Survey{}, &model.Completion{}, &model.SurveySession{}); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	engine := router.New(db)
	addr := ":" + config.Port()
	log.Printf("问而帮 backend listening on %s (dsn=%s)", addr, dsn)
	return engine.Run(addr)
}

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
	"wenbang/internal/service"
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

	if err := db.AutoMigrate(&model.User{}, &model.Survey{}, &model.Completion{}, &model.SurveySession{}, &model.FillReport{}); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := backfillInviteCodes(db); err != nil {
		return fmt.Errorf("backfill invite codes: %w", err)
	}

	engine := router.New(db)
	addr := ":" + config.Port()
	log.Printf("问而帮 backend listening on %s (dsn=%s)", addr, dsn)
	return engine.Run(addr)
}

func backfillInviteCodes(db *gorm.DB) error {
	auth := service.NewAuthService(db)
	var users []model.User
	if err := db.Where("invite_code = ? OR invite_code IS NULL", "").Find(&users).Error; err != nil {
		return err
	}
	for i := range users {
		if _, err := auth.GetUser(users[i].ID); err != nil {
			return err
		}
	}
	return nil
}

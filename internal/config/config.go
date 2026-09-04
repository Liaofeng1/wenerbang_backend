package config

import (
	"os"
	"strconv"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Port() string {
	return getenv("PORT", "8080")
}

func JWTSecret() string {
	return getenv("JWT_SECRET", "dev-secret-change-in-production")
}

func DatabaseDSN() string {
	return getenv("DATABASE_DSN", "file:wenbang.db?cache=shared&mode=rwc")
}

func AppEnv() string {
	return getenv("APP_ENV", "dev")
}

func RegisterBonus() int {
	n, err := strconv.Atoi(getenv("REGISTER_BONUS", "20"))
	if err != nil {
		return 20
	}
	return n
}

// PublishCost is the fixed points deducted when publishing one survey.
func PublishCost() int {
	n, err := strconv.Atoi(getenv("PUBLISH_COST", "5"))
	if err != nil || n <= 0 {
		return 5
	}
	return n
}

// MinAwaySeconds is the minimum time a user must stay away (filling) before complete.
func MinAwaySeconds() int {
	n, err := strconv.Atoi(getenv("MIN_AWAY_SECONDS", "30"))
	if err != nil || n < 0 {
		return 30
	}
	return n
}

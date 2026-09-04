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

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
	n, err := strconv.Atoi(getenv("REGISTER_BONUS", "30"))
	if err != nil {
		return 30
	}
	return n
}

// PublishCost is an optional listing fee. Nonprofit default is 0 (以劳换劳).
func PublishCost() int {
	n, err := strconv.Atoi(getenv("PUBLISH_COST", "0"))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// MinAwaySeconds is the fallback Tmin when a survey has not set one.
func MinAwaySeconds() int {
	n, err := strconv.Atoi(getenv("MIN_AWAY_SECONDS", "120"))
	if err != nil || n < 0 {
		return 120
	}
	return n
}

// InviteReward is granted to both inviter and invitee when invite succeeds.
func InviteReward() int {
	n, err := strconv.Atoi(getenv("INVITE_REWARD", "50"))
	if err != nil || n < 0 {
		return 50
	}
	return n
}

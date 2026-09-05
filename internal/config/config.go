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

// PublishCost is the sunk listing fee (doc §3.2.2: 150).
func PublishCost() int {
	n, err := strconv.Atoi(getenv("PUBLISH_COST", "150"))
	if err != nil || n < 0 {
		return 150
	}
	return n
}

func MinAwaySeconds() int {
	n, err := strconv.Atoi(getenv("MIN_AWAY_SECONDS", "120"))
	if err != nil || n < 0 {
		return 120
	}
	return n
}

func InviteReward() int {
	n, err := strconv.Atoi(getenv("INVITE_REWARD", "50"))
	if err != nil || n < 0 {
		return 50
	}
	return n
}

// PinHourlyCost is 30 points per hour for paid pin (§3.2.2).
func PinHourlyCost() int {
	n, err := strconv.Atoi(getenv("PIN_HOURLY_COST", "30"))
	if err != nil || n < 0 {
		return 30
	}
	return n
}

func BountyMinCount() int {
	n, err := strconv.Atoi(getenv("BOUNTY_MIN_COUNT", "50"))
	if err != nil || n < 1 {
		return 50
	}
	return n
}

func BountyMinPer() int {
	n, err := strconv.Atoi(getenv("BOUNTY_MIN_PER", "10"))
	if err != nil || n < 1 {
		return 10
	}
	return n
}

func TargetingCostPerUser() int {
	n, err := strconv.Atoi(getenv("TARGETING_COST_PER_USER", "5"))
	if err != nil || n < 0 {
		return 5
	}
	return n
}

func TargetingDeliveryMult() int {
	n, err := strconv.Atoi(getenv("TARGETING_DELIVERY_MULT", "2"))
	if err != nil || n < 1 {
		return 2
	}
	return n
}

package model

import (
	"time"
)

type EmailVerification struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Email      string    `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Code       string    `gorm:"size:16;not null" json:"-"`
	ExpiresAt  time.Time `json:"expires_at"`
	Verified   bool      `gorm:"default:false" json:"verified"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ev *EmailVerification) IsExpired() bool {
	return time.Now().After(ev.ExpiresAt)
}

func (ev *EmailVerification) IsValid(code string) bool {
	return !ev.Verified && !ev.IsExpired() && ev.Code == code
}
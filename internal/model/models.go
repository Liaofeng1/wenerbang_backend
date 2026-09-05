package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Nickname     string    `gorm:"size:64" json:"nickname"`
	School       string    `gorm:"size:128" json:"school"`
	DegreeTag    string    `gorm:"size:32;index" json:"degree_tag"`
	Points       int       `gorm:"not null;default:0" json:"points"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

const (
	SurveyStatusOpen   = "open"
	SurveyStatusClosed = "closed"
)

type Survey struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	PublisherID      uint      `gorm:"index;not null" json:"publisher_id"`
	Title            string    `gorm:"size:200;not null" json:"title"`
	Link             string    `gorm:"size:512;not null" json:"link"`
	Description      string    `gorm:"size:1000" json:"description"`
	TargetCount      int       `gorm:"not null" json:"target_count"`
	RewardPoints     int       `gorm:"not null" json:"reward_points"`
	FilledCount      int       `gorm:"not null;default:0" json:"filled_count"`
	Status           string    `gorm:"size:16;not null;default:open;index" json:"status"`
	TargetDegreesRaw string    `gorm:"column:target_degrees;size:512" json:"-"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	TargetDegrees     []string `gorm:"-" json:"target_degrees"`
	PublisherNickname string   `gorm:"-" json:"publisher_nickname,omitempty"`
}

func (s *Survey) BeforeSave(tx *gorm.DB) error {
	raw := strings.Join(s.TargetDegrees, ",")
	s.TargetDegreesRaw = raw
	if tx != nil && tx.Statement != nil {
		tx.Statement.SetColumn("target_degrees", raw)
	}
	return nil
}

func (s *Survey) AfterFind(tx *gorm.DB) error {
	if strings.TrimSpace(s.TargetDegreesRaw) == "" {
		s.TargetDegrees = []string{}
		return nil
	}
	parts := strings.Split(s.TargetDegreesRaw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	s.TargetDegrees = out
	return nil
}

type Completion struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SurveyID     uint      `gorm:"uniqueIndex:idx_survey_user;not null" json:"survey_id"`
	UserID       uint      `gorm:"uniqueIndex:idx_survey_user;not null" json:"user_id"`
	PointsEarned int       `gorm:"not null" json:"points_earned"`
	AwaySeconds  int       `gorm:"not null;default:0" json:"away_seconds"`
	CreatedAt    time.Time `json:"created_at"`

	SurveyTitle string `gorm:"-" json:"survey_title,omitempty"`
}

// SurveySession tracks open → leave → return for anti-spam timing.
type SurveySession struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	SurveyID    uint       `gorm:"uniqueIndex:idx_session_user_survey;not null" json:"survey_id"`
	UserID      uint       `gorm:"uniqueIndex:idx_session_user_survey;not null" json:"user_id"`
	StartedAt   time.Time  `json:"started_at"`
	LeftAt      *time.Time `json:"left_at,omitempty"`
	ReturnedAt  *time.Time `json:"returned_at,omitempty"`
	AwaySeconds int        `gorm:"not null;default:0" json:"away_seconds"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

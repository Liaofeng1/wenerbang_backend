package model

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Nickname     string    `gorm:"size:64" json:"nickname"`
	School       string    `gorm:"size:128" json:"school"`
	Points       int       `gorm:"not null;default:0" json:"points"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

const (
	SurveyStatusOpen   = "open"
	SurveyStatusClosed = "closed"
)

type Survey struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	PublisherID  uint      `gorm:"index;not null" json:"publisher_id"`
	Title        string    `gorm:"size:200;not null" json:"title"`
	Link         string    `gorm:"size:512;not null" json:"link"`
	Description  string    `gorm:"size:1000" json:"description"`
	TargetCount  int       `gorm:"not null" json:"target_count"`
	RewardPoints int       `gorm:"not null" json:"reward_points"`
	FilledCount  int       `gorm:"not null;default:0" json:"filled_count"`
	Status       string    `gorm:"size:16;not null;default:open;index" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	PublisherNickname string `gorm:"-" json:"publisher_nickname,omitempty"`
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

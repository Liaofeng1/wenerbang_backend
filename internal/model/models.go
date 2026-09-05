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
	Major        string    `gorm:"size:128" json:"major"`
	Gender       string    `gorm:"size:16;index" json:"gender"`
	Region       string    `gorm:"size:16;index" json:"region"`
	CityTier     string    `gorm:"size:32;index" json:"city_tier"`
	InviteCode   string    `gorm:"uniqueIndex;size:16;not null" json:"invite_code"`
	InvitedByID  *uint     `gorm:"index" json:"invited_by_id,omitempty"`
	Points       int        `gorm:"not null;default:0" json:"points"`
	Exp          int        `gorm:"not null;default:0;index" json:"exp"`
	LastCheckIn  string     `gorm:"size:16" json:"last_check_in,omitempty"`
	FreePinMonth string     `gorm:"size:7" json:"free_pin_month,omitempty"`
	FreePinUsed  int        `gorm:"not null;default:0" json:"free_pin_used"`
	WarnCount    int        `gorm:"not null;default:0" json:"warn_count"`
	BannedUntil  *time.Time `json:"banned_until,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	Level              int     `gorm:"-" json:"level"`
	LevelTitle         string  `gorm:"-" json:"level_title"`
	ExpToNext          int     `gorm:"-" json:"exp_to_next"`
	NextLevelExp       int     `gorm:"-" json:"next_level_exp"`
	LevelProgressPct   float64 `gorm:"-" json:"level_progress_pct"`
	LevelAtMax         bool    `gorm:"-" json:"level_at_max"`
	CheckedInToday     bool    `gorm:"-" json:"checked_in_today"`
	FreePinRemain      int     `gorm:"-" json:"free_pin_remain"`
	PinDiscountPct     int     `gorm:"-" json:"pin_discount_pct"`
	TargetDiscountPct  int     `gorm:"-" json:"target_discount_pct"`
}

const (
	SurveyStatusOpen   = "open"
	SurveyStatusClosed = "closed"
)

type Survey struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	PublisherID         uint       `gorm:"index;not null" json:"publisher_id"`
	Title               string     `gorm:"size:200;not null" json:"title"`
	Link                string     `gorm:"size:512;not null" json:"link"`
	Description         string     `gorm:"size:1000" json:"description"`
	TargetCount         int        `gorm:"not null" json:"target_count"`
	RewardPoints        int        `gorm:"not null" json:"reward_points"`
	MinFillSeconds      int        `gorm:"not null;default:120" json:"min_fill_seconds"`
	ExpectedFillSeconds int        `gorm:"not null;default:300" json:"expected_fill_seconds"`
	BountyCount         int        `gorm:"not null;default:0" json:"bounty_count"`
	BountyPer           int        `gorm:"not null;default:0" json:"bounty_per"`
	BountyRemain        int        `gorm:"not null;default:0" json:"bounty_remain"`
	FrozenBounty        int        `gorm:"not null;default:0" json:"frozen_bounty"`
	PinHours            int        `gorm:"not null;default:0" json:"pin_hours"`
	PinUntil            *time.Time `json:"pin_until,omitempty"`
	TargetSchool        string     `gorm:"size:128" json:"target_school"`
	TargetMajor         string     `gorm:"size:128" json:"target_major"`
	TargetGender        string     `gorm:"size:16" json:"target_gender"`
	TargetAudienceCount int        `gorm:"not null;default:0" json:"target_audience_count"`
	TargetingReached    int        `gorm:"not null;default:0" json:"targeting_reached"`
	FilledCount         int        `gorm:"not null;default:0" json:"filled_count"`
	ShelfDays           int        `gorm:"not null;default:0" json:"shelf_days"`
	ExpiresAt           *time.Time `gorm:"index" json:"expires_at,omitempty"`
	Status              string     `gorm:"size:16;not null;default:open;index" json:"status"`
	TargetGendersRaw    string     `gorm:"column:target_genders;size:128" json:"-"`
	TargetRegionsRaw    string     `gorm:"column:target_regions;size:64" json:"-"`
	TargetCityTiersRaw  string     `gorm:"column:target_city_tiers;size:128" json:"-"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`

	TargetGenders     []string `gorm:"-" json:"target_genders"`
	TargetRegions     []string `gorm:"-" json:"target_regions"`
	TargetCityTiers   []string `gorm:"-" json:"target_city_tiers"`
	PublisherNickname string   `gorm:"-" json:"publisher_nickname,omitempty"`
	AvgFillSeconds    int      `gorm:"-" json:"avg_fill_seconds,omitempty"`
	EstimatedReward   int      `gorm:"-" json:"estimated_reward,omitempty"`
	IsPinned          bool     `gorm:"-" json:"is_pinned"`
	PinByBounty       bool     `gorm:"-" json:"pin_by_bounty"`
	PinByPaid         bool     `gorm:"-" json:"pin_by_paid"`
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *Survey) BeforeSave(tx *gorm.DB) error {
	s.TargetGendersRaw = strings.Join(s.TargetGenders, ",")
	s.TargetRegionsRaw = strings.Join(s.TargetRegions, ",")
	s.TargetCityTiersRaw = strings.Join(s.TargetCityTiers, ",")
	if tx != nil && tx.Statement != nil {
		tx.Statement.SetColumn("target_genders", s.TargetGendersRaw)
		tx.Statement.SetColumn("target_regions", s.TargetRegionsRaw)
		tx.Statement.SetColumn("target_city_tiers", s.TargetCityTiersRaw)
	}
	return nil
}

func (s *Survey) AfterFind(tx *gorm.DB) error {
	s.TargetGenders = splitCSV(s.TargetGendersRaw)
	s.TargetRegions = splitCSV(s.TargetRegionsRaw)
	s.TargetCityTiers = splitCSV(s.TargetCityTiersRaw)
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

// FillReport is a publisher report against a filler for low-quality answers.
type FillReport struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	SurveyID      uint      `gorm:"uniqueIndex:idx_report_survey_user;not null" json:"survey_id"`
	ReporterID    uint      `gorm:"index;not null" json:"reporter_id"`
	TargetUserID  uint      `gorm:"uniqueIndex:idx_report_survey_user;not null" json:"target_user_id"`
	AwaySeconds   int       `gorm:"not null" json:"away_seconds"`
	RefAvgSeconds int       `gorm:"not null" json:"ref_avg_seconds"`
	Reason        string    `gorm:"size:32;not null" json:"reason"`
	CreatedAt     time.Time `json:"created_at"`
}

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

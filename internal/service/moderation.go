package service

import (
	"errors"
	"math"
	"time"

	"gorm.io/gorm"

	"wenbang/internal/config"
	"wenbang/internal/model"
)

var (
	ErrUserBanned         = errors.New("账号因填写质量警告已暂时封禁，期间不能发问卷或填问卷")
	ErrAlreadyReported    = errors.New("你已举报过该填写者")
	ErrReportNotAbnormal  = errors.New("填写时长未明显偏离参考平均，举报无效，不警告")
	ErrReportNoCompletion = errors.New("该用户未填写此问卷，无法举报")
	ErrReportSelf         = errors.New("不能举报自己")
)

type ReportResult struct {
	Warned        bool       `json:"warned"`
	Reason        string     `json:"reason,omitempty"`
	WarnCount     int        `json:"warn_count"`
	Banned        bool       `json:"banned"`
	BannedUntil   *time.Time `json:"banned_until,omitempty"`
	AwaySeconds   int        `json:"away_seconds"`
	RefAvgSeconds int        `json:"ref_avg_seconds"`
	Message       string     `json:"message"`
}

// ClearExpiredBan lifts an expired ban and resets warn count.
func ClearExpiredBan(tx *gorm.DB, user *model.User) error {
	if user.BannedUntil == nil {
		return nil
	}
	if time.Now().Before(*user.BannedUntil) {
		return nil
	}
	user.BannedUntil = nil
	user.WarnCount = 0
	return tx.Model(user).Updates(map[string]any{
		"banned_until": nil,
		"warn_count":   0,
	}).Error
}

func EnsureNotBanned(tx *gorm.DB, userID uint) error {
	var user model.User
	if err := tx.First(&user, userID).Error; err != nil {
		return err
	}
	if err := ClearExpiredBan(tx, &user); err != nil {
		return err
	}
	if user.BannedUntil != nil && time.Now().Before(*user.BannedUntil) {
		return ErrUserBanned
	}
	return nil
}

func (s *SurveyService) ReportFiller(surveyID, reporterID, targetUserID uint) (*ReportResult, error) {
	var out ReportResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if reporterID == targetUserID {
			return ErrReportSelf
		}
		var survey model.Survey
		if err := tx.First(&survey, surveyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if survey.PublisherID != reporterID {
			return ErrForbidden
		}

		var completion model.Completion
		if err := tx.Where("survey_id = ? AND user_id = ?", surveyID, targetUserID).First(&completion).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReportNoCompletion
			}
			return err
		}

		var existing int64
		if err := tx.Model(&model.FillReport{}).
			Where("survey_id = ? AND target_user_id = ?", surveyID, targetUserID).
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return ErrAlreadyReported
		}

		refAvg := refAvgForReport(tx, surveyID, targetUserID, survey.ExpectedFillSeconds)
		away := completion.AwaySeconds
		fast := config.ReportFastRatio()
		slow := config.ReportSlowRatio()
		reason := ""
		if float64(away) < float64(refAvg)*fast {
			reason = "too_fast"
		} else if float64(away) > float64(refAvg)*slow {
			reason = "too_slow"
		} else {
			return ErrReportNotAbnormal
		}

		report := model.FillReport{
			SurveyID:      surveyID,
			ReporterID:    reporterID,
			TargetUserID:  targetUserID,
			AwaySeconds:   away,
			RefAvgSeconds: refAvg,
			Reason:        reason,
		}
		if err := tx.Create(&report).Error; err != nil {
			return err
		}

		var target model.User
		if err := tx.First(&target, targetUserID).Error; err != nil {
			return err
		}
		if err := ClearExpiredBan(tx, &target); err != nil {
			return err
		}
		if err := tx.First(&target, targetUserID).Error; err != nil {
			return err
		}

		target.WarnCount++
		out.Warned = true
		out.Reason = reason
		out.AwaySeconds = away
		out.RefAvgSeconds = refAvg
		out.WarnCount = target.WarnCount

		if target.WarnCount >= config.WarnLimit() {
			until := time.Now().Add(time.Duration(config.BanDays()) * 24 * time.Hour)
			target.BannedUntil = &until
			out.Banned = true
			out.BannedUntil = &until
			out.Message = "举报生效：已警告并封禁两周"
		} else {
			out.Message = "举报生效：已警告一次"
		}
		return tx.Save(&target).Error
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func refAvgForReport(tx *gorm.DB, surveyID, excludeUserID uint, fallback int) int {
	// Prefer other fillers' average; never use the reported user's own time as the baseline.
	var avg *float64
	err := tx.Model(&model.Completion{}).
		Where("survey_id = ? AND user_id <> ?", surveyID, excludeUserID).
		Select("AVG(away_seconds)").
		Scan(&avg).Error
	if err == nil && avg != nil && *avg > 0 {
		return int(math.Round(*avg))
	}
	if fallback <= 0 {
		return 300
	}
	return fallback
}

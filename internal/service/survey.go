package service

import (
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"

	"wenbang/internal/config"
	"wenbang/internal/model"
	"wenbang/internal/points"
)

var (
	ErrNotFound         = errors.New("资源不存在")
	ErrForbidden        = errors.New("无权操作")
	ErrInsufficientPts  = errors.New("积分不足")
	ErrAlreadyCompleted = errors.New("你已填写过该问卷")
	ErrOwnSurvey        = errors.New("不能填写自己发布的问卷")
	ErrSurveyClosed     = errors.New("问卷已结束")
	ErrBadSurveyInput   = errors.New("问卷参数不合法")
	ErrNeedOpenFirst    = errors.New("请先点击「打开问卷」再填写")
	ErrAwayTooShort     = errors.New("填写时长未达到发布者设置的最低停留时间，暂不能获得积分")
)

type SurveyService struct {
	db *gorm.DB
}

func NewSurveyService(db *gorm.DB) *SurveyService {
	return &SurveyService{db: db}
}

type CreateSurveyInput struct {
	Title               string `json:"title"`
	Link                string `json:"link"`
	Description         string `json:"description"`
	TargetCount         int    `json:"target_count"`
	MinFillSeconds      int    `json:"min_fill_seconds"`
	ExpectedFillSeconds int    `json:"expected_fill_seconds"`
	BountyCount         int    `json:"bounty_count"`
	BountyPer           int    `json:"bounty_per"`
}

func (s *SurveyService) Create(publisherID uint, in CreateSurveyInput) (*model.Survey, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Link = strings.TrimSpace(in.Link)
	in.Description = strings.TrimSpace(in.Description)
	if in.Title == "" || in.Link == "" || in.TargetCount <= 0 {
		return nil, ErrBadSurveyInput
	}
	if in.MinFillSeconds <= 0 {
		in.MinFillSeconds = points.DefaultTmin
	}
	if in.ExpectedFillSeconds <= 0 {
		in.ExpectedFillSeconds = points.DefaultTavg
	}
	if in.MinFillSeconds < 10 || in.MinFillSeconds > 7200 {
		return nil, ErrBadSurveyInput
	}
	if in.ExpectedFillSeconds < in.MinFillSeconds || in.ExpectedFillSeconds > 7200 {
		return nil, ErrBadSurveyInput
	}
	if in.BountyCount < 0 || in.BountyPer < 0 {
		return nil, ErrBadSurveyInput
	}
	if in.BountyCount > in.TargetCount {
		in.BountyCount = in.TargetCount
	}
	if in.BountyCount > 0 && in.BountyPer <= 0 {
		return nil, ErrBadSurveyInput
	}
	if in.BountyCount == 0 {
		in.BountyPer = 0
	}

	listing := config.PublishCost()
	freeze := in.BountyCount * in.BountyPer
	need := listing + freeze
	peak := points.PeakReward(in.ExpectedFillSeconds)

	var survey model.Survey
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, publisherID).Error; err != nil {
			return err
		}
		if user.Points < need {
			return ErrInsufficientPts
		}
		if need > 0 {
			user.Points -= need
			if err := tx.Save(&user).Error; err != nil {
				return err
			}
		}
		survey = model.Survey{
			PublisherID:         publisherID,
			Title:               in.Title,
			Link:                in.Link,
			Description:         in.Description,
			TargetCount:         in.TargetCount,
			RewardPoints:        peak,
			MinFillSeconds:      in.MinFillSeconds,
			ExpectedFillSeconds: in.ExpectedFillSeconds,
			BountyCount:         in.BountyCount,
			BountyPer:           in.BountyPer,
			BountyRemain:        in.BountyCount,
			FrozenBounty:        freeze,
			FilledCount:         0,
			Status:              model.SurveyStatusOpen,
		}
		return tx.Create(&survey).Error
	})
	if err != nil {
		return nil, err
	}
	survey.EstimatedReward = peak
	survey.AvgFillSeconds = in.ExpectedFillSeconds
	return &survey, nil
}

func (s *SurveyService) ListOpen(viewerID uint) ([]model.Survey, error) {
	var list []model.Survey
	q := s.db.Where("status = ?", model.SurveyStatusOpen).Order("id desc")
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	s.attachPublisherNames(list)
	s.attachFillStats(list)
	return list, nil
}

func (s *SurveyService) ListMine(userID uint) ([]model.Survey, error) {
	var list []model.Survey
	if err := s.db.Where("publisher_id = ?", userID).Order("id desc").Find(&list).Error; err != nil {
		return nil, err
	}
	s.attachFillStats(list)
	return list, nil
}

func (s *SurveyService) Get(id uint) (*model.Survey, error) {
	var survey model.Survey
	if err := s.db.First(&survey, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var pub model.User
	if err := s.db.Select("nickname").First(&pub, survey.PublisherID).Error; err == nil {
		survey.PublisherNickname = pub.Nickname
	}
	tmp := []model.Survey{survey}
	s.attachFillStats(tmp)
	survey = tmp[0]
	return &survey, nil
}

type SessionView struct {
	SurveyID       uint `json:"survey_id"`
	AwaySeconds    int  `json:"away_seconds"`
	MinAwaySeconds int  `json:"min_away_seconds"`
	Ready          bool `json:"ready"`
}

func (s *SurveyService) ensureFillable(tx *gorm.DB, surveyID, userID uint) (*model.Survey, error) {
	var survey model.Survey
	if err := tx.First(&survey, surveyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if survey.Status != model.SurveyStatusOpen {
		return nil, ErrSurveyClosed
	}
	if survey.PublisherID == userID {
		return nil, ErrOwnSurvey
	}
	var existing int64
	if err := tx.Model(&model.Completion{}).
		Where("survey_id = ? AND user_id = ?", surveyID, userID).
		Count(&existing).Error; err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrAlreadyCompleted
	}
	return &survey, nil
}

func minFillOf(survey *model.Survey) int {
	if survey != nil && survey.MinFillSeconds > 0 {
		return survey.MinFillSeconds
	}
	n := config.MinAwaySeconds()
	if n <= 0 {
		return points.DefaultTmin
	}
	return n
}

func (s *SurveyService) Start(surveyID, userID uint) (*SessionView, error) {
	var view SessionView
	err := s.db.Transaction(func(tx *gorm.DB) error {
		survey, err := s.ensureFillable(tx, surveyID, userID)
		if err != nil {
			return err
		}
		now := time.Now()
		var session model.SurveySession
		err = tx.Where("survey_id = ? AND user_id = ?", surveyID, userID).First(&session).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			session = model.SurveySession{
				SurveyID:  surveyID,
				UserID:    userID,
				StartedAt: now,
			}
			if err := tx.Create(&session).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			session.LeftAt = nil
			if err := tx.Save(&session).Error; err != nil {
				return err
			}
		}
		view = toSessionView(&session, minFillOf(survey))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (s *SurveyService) Leave(surveyID, userID uint) (*SessionView, error) {
	var view SessionView
	err := s.db.Transaction(func(tx *gorm.DB) error {
		survey, err := s.ensureFillable(tx, surveyID, userID)
		if err != nil {
			return err
		}
		var session model.SurveySession
		if err := tx.Where("survey_id = ? AND user_id = ?", surveyID, userID).First(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNeedOpenFirst
			}
			return err
		}
		if session.LeftAt == nil {
			now := time.Now()
			session.LeftAt = &now
			if err := tx.Save(&session).Error; err != nil {
				return err
			}
		}
		view = toSessionView(&session, minFillOf(survey))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (s *SurveyService) Return(surveyID, userID uint) (*SessionView, error) {
	var view SessionView
	err := s.db.Transaction(func(tx *gorm.DB) error {
		survey, err := s.ensureFillable(tx, surveyID, userID)
		if err != nil {
			return err
		}
		var session model.SurveySession
		if err := tx.Where("survey_id = ? AND user_id = ?", surveyID, userID).First(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNeedOpenFirst
			}
			return err
		}
		if session.LeftAt != nil {
			now := time.Now()
			delta := int(now.Sub(*session.LeftAt).Seconds())
			if delta < 0 {
				delta = 0
			}
			session.AwaySeconds += delta
			session.LeftAt = nil
			session.ReturnedAt = &now
			if err := tx.Save(&session).Error; err != nil {
				return err
			}
		}
		view = toSessionView(&session, minFillOf(survey))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (s *SurveyService) GetSession(surveyID, userID uint) (*SessionView, error) {
	var survey model.Survey
	if err := s.db.First(&survey, surveyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	minAway := minFillOf(&survey)
	var session model.SurveySession
	if err := s.db.Where("survey_id = ? AND user_id = ?", surveyID, userID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &SessionView{
				SurveyID:       surveyID,
				AwaySeconds:    0,
				MinAwaySeconds: minAway,
				Ready:          false,
			}, nil
		}
		return nil, err
	}
	view := toSessionView(&session, minAway)
	return &view, nil
}

func toSessionView(session *model.SurveySession, minAway int) SessionView {
	away := session.AwaySeconds
	if session.LeftAt != nil {
		delta := int(time.Since(*session.LeftAt).Seconds())
		if delta > 0 {
			away += delta
		}
	}
	if minAway <= 0 {
		minAway = points.DefaultTmin
	}
	return SessionView{
		SurveyID:       session.SurveyID,
		AwaySeconds:    away,
		MinAwaySeconds: minAway,
		Ready:          away >= minAway,
	}
}

func (s *SurveyService) Complete(surveyID, userID uint) (*model.Completion, error) {
	var completion model.Completion
	err := s.db.Transaction(func(tx *gorm.DB) error {
		survey, err := s.ensureFillable(tx, surveyID, userID)
		if err != nil {
			return err
		}

		var session model.SurveySession
		if err := tx.Where("survey_id = ? AND user_id = ?", surveyID, userID).First(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNeedOpenFirst
			}
			return err
		}
		if session.LeftAt != nil {
			now := time.Now()
			delta := int(now.Sub(*session.LeftAt).Seconds())
			if delta > 0 {
				session.AwaySeconds += delta
			}
			session.LeftAt = nil
			session.ReturnedAt = &now
		}
		minFill := minFillOf(survey)
		if session.AwaySeconds < minFill {
			return ErrAwayTooShort
		}

		avg := avgFillSeconds(tx, surveyID, survey.ExpectedFillSeconds)
		base := points.FillReward(session.AwaySeconds, avg, minFill)
		bounty := 0
		if survey.BountyRemain > 0 && survey.BountyPer > 0 {
			bounty = survey.BountyPer
			survey.BountyRemain--
			survey.FrozenBounty -= bounty
			if survey.FrozenBounty < 0 {
				survey.FrozenBounty = 0
			}
		}

		var filler model.User
		if err := tx.First(&filler, userID).Error; err != nil {
			return err
		}
		earned := base + bounty
		filler.Points += earned
		if err := tx.Save(&filler).Error; err != nil {
			return err
		}

		completion = model.Completion{
			SurveyID:     surveyID,
			UserID:       userID,
			PointsEarned: earned,
			AwaySeconds:  session.AwaySeconds,
		}
		if err := tx.Create(&completion).Error; err != nil {
			return err
		}

		survey.FilledCount++
		if survey.FilledCount >= survey.TargetCount {
			survey.Status = model.SurveyStatusClosed
			if err := refundFrozenBounty(tx, survey); err != nil {
				return err
			}
		}
		if err := tx.Save(survey).Error; err != nil {
			return err
		}
		return tx.Save(&session).Error
	})
	if err != nil {
		return nil, err
	}
	return &completion, nil
}

func refundFrozenBounty(tx *gorm.DB, survey *model.Survey) error {
	if survey.FrozenBounty <= 0 {
		survey.BountyRemain = 0
		return nil
	}
	if err := tx.Model(&model.User{}).
		Where("id = ?", survey.PublisherID).
		Update("points", gorm.Expr("points + ?", survey.FrozenBounty)).Error; err != nil {
		return err
	}
	survey.FrozenBounty = 0
	survey.BountyRemain = 0
	return nil
}

func avgFillSeconds(tx *gorm.DB, surveyID uint, fallback int) int {
	if fallback <= 0 {
		fallback = points.DefaultTavg
	}
	var avg sql.NullFloat64
	if err := tx.Model(&model.Completion{}).
		Where("survey_id = ?", surveyID).
		Select("AVG(away_seconds)").
		Scan(&avg).Error; err != nil || !avg.Valid || avg.Float64 <= 0 {
		return fallback
	}
	return int(math.Round(avg.Float64))
}

func (s *SurveyService) ListMyCompletions(userID uint) ([]model.Completion, error) {
	var list []model.Completion
	if err := s.db.Where("user_id = ?", userID).Order("id desc").Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return list, nil
	}
	ids := make([]uint, 0, len(list))
	for _, c := range list {
		ids = append(ids, c.SurveyID)
	}
	var surveys []model.Survey
	_ = s.db.Select("id, title").Where("id IN ?", ids).Find(&surveys)
	titleMap := map[uint]string{}
	for _, sv := range surveys {
		titleMap[sv.ID] = sv.Title
	}
	for i := range list {
		list[i].SurveyTitle = titleMap[list[i].SurveyID]
	}
	return list, nil
}

func (s *SurveyService) attachPublisherNames(list []model.Survey) {
	if len(list) == 0 {
		return
	}
	ids := make([]uint, 0, len(list))
	seen := map[uint]struct{}{}
	for _, item := range list {
		if _, ok := seen[item.PublisherID]; ok {
			continue
		}
		seen[item.PublisherID] = struct{}{}
		ids = append(ids, item.PublisherID)
	}
	var users []model.User
	_ = s.db.Select("id, nickname").Where("id IN ?", ids).Find(&users)
	nameMap := map[uint]string{}
	for _, u := range users {
		nameMap[u.ID] = u.Nickname
	}
	for i := range list {
		list[i].PublisherNickname = nameMap[list[i].PublisherID]
	}
}

func (s *SurveyService) attachFillStats(list []model.Survey) {
	if len(list) == 0 {
		return
	}
	ids := make([]uint, 0, len(list))
	for _, item := range list {
		ids = append(ids, item.ID)
	}
	type row struct {
		SurveyID uint
		AvgAway  float64
	}
	var rows []row
	_ = s.db.Model(&model.Completion{}).
		Select("survey_id, AVG(away_seconds) as avg_away").
		Where("survey_id IN ?", ids).
		Group("survey_id").
		Scan(&rows)
	avgMap := map[uint]int{}
	for _, r := range rows {
		if r.AvgAway > 0 {
			avgMap[r.SurveyID] = int(math.Round(r.AvgAway))
		}
	}
	for i := range list {
		avg := list[i].ExpectedFillSeconds
		if avg <= 0 {
			avg = points.DefaultTavg
		}
		if v, ok := avgMap[list[i].ID]; ok && v > 0 {
			avg = v
		}
		peak := points.PeakReward(avg)
		list[i].AvgFillSeconds = avg
		list[i].EstimatedReward = peak
		list[i].RewardPoints = peak
	}
}

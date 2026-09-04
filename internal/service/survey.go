package service

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"wenbang/internal/config"
	"wenbang/internal/model"
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
	ErrAwayTooShort     = errors.New("离开填写时间过短，请认真作答后再提交")
)

type SurveyService struct {
	db *gorm.DB
}

func NewSurveyService(db *gorm.DB) *SurveyService {
	return &SurveyService{db: db}
}

type CreateSurveyInput struct {
	Title        string `json:"title"`
	Link         string `json:"link"`
	Description  string `json:"description"`
	TargetCount  int    `json:"target_count"`
	RewardPoints int    `json:"reward_points"`
}

func (s *SurveyService) Create(publisherID uint, in CreateSurveyInput) (*model.Survey, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Link = strings.TrimSpace(in.Link)
	in.Description = strings.TrimSpace(in.Description)
	if in.Title == "" || in.Link == "" || in.TargetCount <= 0 || in.RewardPoints <= 0 {
		return nil, ErrBadSurveyInput
	}
	cost := config.PublishCost() // fixed publish cost (default 5)

	var survey model.Survey
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, publisherID).Error; err != nil {
			return err
		}
		if user.Points < cost {
			return ErrInsufficientPts
		}
		user.Points -= cost
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		survey = model.Survey{
			PublisherID:  publisherID,
			Title:        in.Title,
			Link:         in.Link,
			Description:  in.Description,
			TargetCount:  in.TargetCount,
			RewardPoints: in.RewardPoints,
			FilledCount:  0,
			Status:       model.SurveyStatusOpen,
		}
		return tx.Create(&survey).Error
	})
	if err != nil {
		return nil, err
	}
	return &survey, nil
}

func (s *SurveyService) ListOpen(excludeUserID uint) ([]model.Survey, error) {
	var list []model.Survey
	q := s.db.Where("status = ?", model.SurveyStatusOpen).Order("id desc")
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	s.attachPublisherNames(list)
	return list, nil
}

func (s *SurveyService) ListMine(userID uint) ([]model.Survey, error) {
	var list []model.Survey
	if err := s.db.Where("publisher_id = ?", userID).Order("id desc").Find(&list).Error; err != nil {
		return nil, err
	}
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
	s.attachPublisherNames([]model.Survey{survey})
	var pub model.User
	if err := s.db.Select("nickname").First(&pub, survey.PublisherID).Error; err == nil {
		survey.PublisherNickname = pub.Nickname
	}
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

func (s *SurveyService) Start(surveyID, userID uint) (*SessionView, error) {
	var view SessionView
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.ensureFillable(tx, surveyID, userID); err != nil {
			return err
		}
		now := time.Now()
		var session model.SurveySession
		err := tx.Where("survey_id = ? AND user_id = ?", surveyID, userID).First(&session).Error
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
			// reopen: keep accumulated away, clear in-progress leave
			session.LeftAt = nil
			if err := tx.Save(&session).Error; err != nil {
				return err
			}
		}
		view = toSessionView(&session)
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
		if _, err := s.ensureFillable(tx, surveyID, userID); err != nil {
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
		view = toSessionView(&session)
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
		if _, err := s.ensureFillable(tx, surveyID, userID); err != nil {
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
		view = toSessionView(&session)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (s *SurveyService) GetSession(surveyID, userID uint) (*SessionView, error) {
	var session model.SurveySession
	if err := s.db.Where("survey_id = ? AND user_id = ?", surveyID, userID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &SessionView{
				SurveyID:       surveyID,
				AwaySeconds:    0,
				MinAwaySeconds: config.MinAwaySeconds(),
				Ready:          false,
			}, nil
		}
		return nil, err
	}
	view := toSessionView(&session)
	return &view, nil
}

func toSessionView(session *model.SurveySession) SessionView {
	away := session.AwaySeconds
	if session.LeftAt != nil {
		delta := int(time.Since(*session.LeftAt).Seconds())
		if delta > 0 {
			away += delta
		}
	}
	minAway := config.MinAwaySeconds()
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
		// If still away, close the leave interval now.
		if session.LeftAt != nil {
			now := time.Now()
			delta := int(now.Sub(*session.LeftAt).Seconds())
			if delta > 0 {
				session.AwaySeconds += delta
			}
			session.LeftAt = nil
			session.ReturnedAt = &now
		}
		if session.AwaySeconds < config.MinAwaySeconds() {
			return ErrAwayTooShort
		}

		var filler model.User
		if err := tx.First(&filler, userID).Error; err != nil {
			return err
		}
		filler.Points += survey.RewardPoints
		if err := tx.Save(&filler).Error; err != nil {
			return err
		}

		completion = model.Completion{
			SurveyID:     surveyID,
			UserID:       userID,
			PointsEarned: survey.RewardPoints,
			AwaySeconds:  session.AwaySeconds,
		}
		if err := tx.Create(&completion).Error; err != nil {
			return err
		}

		survey.FilledCount++
		if survey.FilledCount >= survey.TargetCount {
			survey.Status = model.SurveyStatusClosed
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

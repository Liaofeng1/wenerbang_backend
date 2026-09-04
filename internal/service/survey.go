package service

import (
	"errors"
	"strings"

	"gorm.io/gorm"

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
	cost := in.TargetCount * in.RewardPoints

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

func (s *SurveyService) Complete(surveyID, userID uint) (*model.Completion, error) {
	var completion model.Completion
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var survey model.Survey
		if err := tx.First(&survey, surveyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if survey.Status != model.SurveyStatusOpen {
			return ErrSurveyClosed
		}
		if survey.PublisherID == userID {
			return ErrOwnSurvey
		}

		var existing int64
		if err := tx.Model(&model.Completion{}).
			Where("survey_id = ? AND user_id = ?", surveyID, userID).
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return ErrAlreadyCompleted
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
		}
		if err := tx.Create(&completion).Error; err != nil {
			return err
		}

		survey.FilledCount++
		if survey.FilledCount >= survey.TargetCount {
			survey.Status = model.SurveyStatusClosed
		}
		return tx.Save(&survey).Error
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

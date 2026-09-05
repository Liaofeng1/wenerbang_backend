package service

import (
	"database/sql"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"wenbang/internal/config"
	"wenbang/internal/level"
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
	ErrProfileMismatch  = errors.New("该问卷未面向你的用户画像（性别/城市）")
	ErrAudienceMismatch = errors.New("该问卷定向投放，与你的画像不符")
	ErrDeliveryFull     = errors.New("该定向问卷投放名额已满")
)

type SurveyService struct {
	db *gorm.DB
}

func NewSurveyService(db *gorm.DB) *SurveyService {
	return &SurveyService{db: db}
}

type CreateSurveyInput struct {
	Title               string   `json:"title"`
	Link                string   `json:"link"`
	Description         string   `json:"description"`
	TargetCount         int      `json:"target_count"`
	MinFillSeconds      int      `json:"min_fill_seconds"`
	ExpectedFillSeconds int      `json:"expected_fill_seconds"`
	BountyCount         int      `json:"bounty_count"`
	BountyPer           int      `json:"bounty_per"`
	PinHours            int      `json:"pin_hours"`
	TargetSchool        string   `json:"target_school"`
	TargetMajor         string   `json:"target_major"`
	TargetGender        string   `json:"target_gender"`
	TargetAudienceCount int      `json:"target_audience_count"`
	TargetGenders       []string `json:"target_genders"`
	TargetRegions       []string `json:"target_regions"`
	TargetCityTiers     []string `json:"target_city_tiers"`
	ShelfDays           int      `json:"shelf_days"`
}

func validPinHours(h int) bool {
	return h == 0 || h == 4 || h == 6 || h == 8
}

func (s *SurveyService) expireDueSurveys(tx *gorm.DB) error {
	now := time.Now()
	var due []model.Survey
	if err := tx.Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", model.SurveyStatusOpen, now).
		Find(&due).Error; err != nil {
		return err
	}
	for i := range due {
		sv := &due[i]
		sv.Status = model.SurveyStatusClosed
		if err := refundFrozenBounty(tx, sv); err != nil {
			return err
		}
		if err := tx.Save(sv).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *SurveyService) closeIfExpired(tx *gorm.DB, sv *model.Survey) error {
	if sv == nil || sv.Status != model.SurveyStatusOpen || sv.ExpiresAt == nil {
		return nil
	}
	if time.Now().Before(*sv.ExpiresAt) {
		return nil
	}
	sv.Status = model.SurveyStatusClosed
	if err := refundFrozenBounty(tx, sv); err != nil {
		return err
	}
	return tx.Save(sv).Error
}

func (s *SurveyService) Create(publisherID uint, in CreateSurveyInput) (*model.Survey, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Link = strings.TrimSpace(in.Link)
	in.Description = strings.TrimSpace(in.Description)
	in.TargetSchool = strings.TrimSpace(in.TargetSchool)
	in.TargetMajor = strings.TrimSpace(in.TargetMajor)
	in.TargetGender = strings.TrimSpace(in.TargetGender)
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
	if !validPinHours(in.PinHours) {
		return nil, ErrBadSurveyInput
	}
	maxShelf := config.MaxShelfDays()
	if in.ShelfDays < 1 || in.ShelfDays > maxShelf {
		return nil, ErrBadSurveyInput
	}

	// Bounty: optional; if on → min 50 slots, min 10 pts each (§3.2.2)
	if in.BountyCount < 0 || in.BountyPer < 0 {
		return nil, ErrBadSurveyInput
	}
	if in.BountyCount == 0 {
		in.BountyPer = 0
	} else {
		if in.BountyCount < config.BountyMinCount() || in.BountyPer < config.BountyMinPer() {
			return nil, ErrBadSurveyInput
		}
	}

	if in.TargetMajor != "" && !validDiscipline(in.TargetMajor) {
		return nil, ErrBadSurveyInput
	}
	if in.TargetGender != "" && !model.IsValidGender(in.TargetGender) {
		return nil, ErrBadSurveyInput
	}

	hasTargetFilter := in.TargetSchool != "" || in.TargetMajor != "" || in.TargetGender != ""
	if in.TargetAudienceCount < 0 {
		return nil, ErrBadSurveyInput
	}
	if hasTargetFilter && in.TargetAudienceCount <= 0 {
		return nil, ErrBadSurveyInput
	}
	if in.TargetAudienceCount > 0 && !hasTargetFilter {
		return nil, ErrBadSurveyInput
	}

	genders, ok := model.NormalizeGenders(in.TargetGenders)
	if !ok {
		return nil, ErrBadSurveyInput
	}
	regions, ok := model.NormalizeRegions(in.TargetRegions)
	if !ok {
		return nil, ErrBadSurveyInput
	}
	tiers, ok := model.NormalizeCityTiers(in.TargetCityTiers)
	if !ok {
		return nil, ErrBadSurveyInput
	}

	listing := config.PublishCost()
	bountyFreeze := in.BountyCount * in.BountyPer
	rawPinCost := in.PinHours * config.PinHourlyCost()
	rawTargetingCost := in.TargetAudienceCount * config.TargetingCostPerUser()
	peak := points.PeakReward(in.ExpectedFillSeconds)

	var pinUntil *time.Time
	if in.PinHours > 0 {
		t := time.Now().Add(time.Duration(in.PinHours) * time.Hour)
		pinUntil = &t
	}

	var survey model.Survey
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := EnsureNotBanned(tx, publisherID); err != nil {
			return err
		}
		var user model.User
		if err := tx.First(&user, publisherID).Error; err != nil {
			return err
		}
		lv := level.LevelOf(user.Exp)
		pinCost := level.ApplyPinDiscount(rawPinCost, lv)
		targetingCost := level.ApplyTargetingDiscount(rawTargetingCost, lv)

		// Monthly free pin: one use waives pin fee entirely.
		month := level.MonthKey()
		if user.FreePinMonth != month {
			user.FreePinMonth = month
			user.FreePinUsed = 0
		}
		usedFree := false
		if in.PinHours > 0 && pinCost > 0 {
			remain := level.FreePinsAllowed(lv) - user.FreePinUsed
			if remain > 0 {
				pinCost = 0
				user.FreePinUsed++
				usedFree = true
			}
		}
		_ = usedFree

		need := listing + bountyFreeze + pinCost + targetingCost
		if user.Points < need {
			return ErrInsufficientPts
		}
		user.Points -= need
		level.AddExp(&user, level.XPPublish)
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		expires := time.Now().Add(time.Duration(in.ShelfDays) * 24 * time.Hour)
		expiresAt := &expires
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
			FrozenBounty:        bountyFreeze,
			PinHours:            in.PinHours,
			PinUntil:            pinUntil,
			TargetSchool:        in.TargetSchool,
			TargetMajor:         in.TargetMajor,
			TargetGender:        in.TargetGender,
			TargetAudienceCount: in.TargetAudienceCount,
			TargetingReached:    0,
			FilledCount:         0,
			ShelfDays:           in.ShelfDays,
			ExpiresAt:           expiresAt,
			Status:              model.SurveyStatusOpen,
			TargetGenders:       genders,
			TargetGendersRaw:    strings.Join(genders, ","),
			TargetRegions:       regions,
			TargetRegionsRaw:    strings.Join(regions, ","),
			TargetCityTiers:     tiers,
			TargetCityTiersRaw:  strings.Join(tiers, ","),
		}
		return tx.Create(&survey).Error
	})
	if err != nil {
		return nil, err
	}
	survey.EstimatedReward = peak
	survey.AvgFillSeconds = in.ExpectedFillSeconds
	attachPinFlags([]*model.Survey{&survey})
	return &survey, nil
}

func (s *SurveyService) ListOpen(viewerID uint) ([]model.Survey, error) {
	if err := s.expireDueSurveys(s.db); err != nil {
		return nil, err
	}
	var viewer model.User
	if err := s.db.First(&viewer, viewerID).Error; err != nil {
		return nil, err
	}

	var list []model.Survey
	if err := s.db.Where("status = ?", model.SurveyStatusOpen).Find(&list).Error; err != nil {
		return nil, err
	}

	filtered := make([]model.Survey, 0, len(list))
	for i := range list {
		sv := &list[i]
		if sv.PublisherID == viewerID {
			filtered = append(filtered, *sv)
			continue
		}
		if !sv.AllowsUser(&viewer) {
			continue
		}
		if !targetingVisible(sv, &viewer) {
			continue
		}
		if targetingFull(sv) {
			continue
		}
		filtered = append(filtered, *sv)
	}
	attachPinFlagsPtr(filtered)
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].IsPinned != filtered[j].IsPinned {
			return filtered[i].IsPinned
		}
		return filtered[i].ID > filtered[j].ID
	})
	s.attachPublisherNames(filtered)
	s.attachFillStats(filtered)
	return filtered, nil
}

func (s *SurveyService) ListMine(userID uint) ([]model.Survey, error) {
	if err := s.expireDueSurveys(s.db); err != nil {
		return nil, err
	}
	var list []model.Survey
	if err := s.db.Where("publisher_id = ?", userID).Order("id desc").Find(&list).Error; err != nil {
		return nil, err
	}
	attachPinFlagsPtr(list)
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
	if err := s.closeIfExpired(s.db, &survey); err != nil {
		return nil, err
	}
	var pub model.User
	if err := s.db.Select("nickname").First(&pub, survey.PublisherID).Error; err == nil {
		survey.PublisherNickname = pub.Nickname
	}
	tmp := []model.Survey{survey}
	s.attachFillStats(tmp)
	attachPinFlagsPtr(tmp)
	survey = tmp[0]
	return &survey, nil
}

type SessionView struct {
	SurveyID       uint `json:"survey_id"`
	AwaySeconds    int  `json:"away_seconds"`
	MinAwaySeconds int  `json:"min_away_seconds"`
	Ready          bool `json:"ready"`
}

func targetingEnabled(sv *model.Survey) bool {
	return sv.TargetAudienceCount > 0
}

func targetingFull(sv *model.Survey) bool {
	if !targetingEnabled(sv) {
		return false
	}
	capN := sv.TargetAudienceCount * config.TargetingDeliveryMult()
	return sv.TargetingReached >= capN
}

func userMatchesTarget(sv *model.Survey, u *model.User) bool {
	if sv.TargetSchool != "" && !strings.EqualFold(strings.TrimSpace(u.School), sv.TargetSchool) {
		return false
	}
	if sv.TargetMajor != "" && !strings.EqualFold(strings.TrimSpace(u.Major), sv.TargetMajor) {
		return false
	}
	if sv.TargetGender != "" && !strings.EqualFold(strings.TrimSpace(u.Gender), sv.TargetGender) {
		return false
	}
	return true
}

func targetingVisible(sv *model.Survey, u *model.User) bool {
	if !targetingEnabled(sv) {
		return true
	}
	return userMatchesTarget(sv, u)
}

func paidPinActive(sv *model.Survey) bool {
	return sv.PinUntil != nil && sv.PinUntil.After(time.Now())
}

func bountyPinActive(sv *model.Survey) bool {
	return sv.BountyRemain > 0 && sv.BountyPer > 0
}

func attachPinFlags(list []*model.Survey) {
	for _, sv := range list {
		sv.PinByBounty = bountyPinActive(sv)
		sv.PinByPaid = paidPinActive(sv)
		sv.IsPinned = sv.PinByBounty || sv.PinByPaid
	}
}

func attachPinFlagsPtr(list []model.Survey) {
	for i := range list {
		list[i].PinByBounty = bountyPinActive(&list[i])
		list[i].PinByPaid = paidPinActive(&list[i])
		list[i].IsPinned = list[i].PinByBounty || list[i].PinByPaid
	}
}

func (s *SurveyService) ensureFillable(tx *gorm.DB, surveyID, userID uint) (*model.Survey, error) {
	if err := EnsureNotBanned(tx, userID); err != nil {
		return nil, err
	}
	var survey model.Survey
	if err := tx.First(&survey, surveyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := s.closeIfExpired(tx, &survey); err != nil {
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

	var user model.User
	if err := tx.First(&user, userID).Error; err != nil {
		return nil, err
	}
	if !survey.AllowsUser(&user) {
		return nil, ErrProfileMismatch
	}
	if targetingEnabled(&survey) {
		if !userMatchesTarget(&survey, &user) {
			return nil, ErrAudienceMismatch
		}
		if targetingFull(&survey) {
			return nil, ErrDeliveryFull
		}
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
		level.AddExp(&filler, level.XPFill)
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

		if targetingEnabled(survey) && userMatchesTarget(survey, &filler) {
			survey.TargetingReached++
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

type CompletionDetail struct {
	UserID      uint      `json:"user_id"`
	Nickname    string    `json:"nickname"`
	Gender      string    `json:"gender"`
	Region      string    `json:"region"`
	CityTier    string    `json:"city_tier"`
	School      string    `json:"school"`
	Major       string    `json:"major"`
	AwaySeconds int       `json:"away_seconds"`
	CompletedAt time.Time `json:"completed_at"`
}

type SurveyStats struct {
	SurveyID       uint               `json:"survey_id"`
	Title          string             `json:"title"`
	Status         string             `json:"status"`
	FilledCount    int                `json:"filled_count"`
	TargetCount    int                `json:"target_count"`
	MinFillSeconds int                `json:"min_fill_seconds"`
	MinAwaySeconds int                `json:"min_away_seconds"`
	GenderCounts   map[string]int     `json:"gender_counts"`
	RegionCounts   map[string]int     `json:"region_counts"`
	CityTierCounts map[string]int     `json:"city_tier_counts"`
	AvgAwaySeconds float64            `json:"avg_away_seconds"`
	Completions    []CompletionDetail `json:"completions"`
}

func (s *SurveyService) Stats(surveyID, requesterID uint) (*SurveyStats, error) {
	var survey model.Survey
	if err := s.db.First(&survey, surveyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if survey.PublisherID != requesterID {
		return nil, ErrForbidden
	}

	var completions []model.Completion
	if err := s.db.Where("survey_id = ?", surveyID).Order("id asc").Find(&completions).Error; err != nil {
		return nil, err
	}

	minFill := minFillOf(&survey)
	stats := &SurveyStats{
		SurveyID:       survey.ID,
		Title:          survey.Title,
		Status:         survey.Status,
		FilledCount:    survey.FilledCount,
		TargetCount:    survey.TargetCount,
		MinFillSeconds: minFill,
		MinAwaySeconds: minFill,
		GenderCounts:   map[string]int{},
		RegionCounts:   map[string]int{},
		CityTierCounts: map[string]int{},
		Completions:    make([]CompletionDetail, 0, len(completions)),
	}
	if len(completions) == 0 {
		return stats, nil
	}

	userIDs := make([]uint, 0, len(completions))
	for _, c := range completions {
		userIDs = append(userIDs, c.UserID)
	}
	var users []model.User
	_ = s.db.Where("id IN ?", userIDs).Find(&users)
	userMap := map[uint]model.User{}
	for _, u := range users {
		userMap[u.ID] = u
	}

	var awaySum int
	for _, c := range completions {
		u := userMap[c.UserID]
		gender := u.Gender
		if gender == "" {
			gender = "未知"
		}
		region := u.Region
		if region == "" {
			region = "未知"
		}
		tier := u.CityTier
		if tier == "" {
			tier = "未知"
		}
		stats.GenderCounts[gender]++
		stats.RegionCounts[region]++
		stats.CityTierCounts[tier]++
		awaySum += c.AwaySeconds
		stats.Completions = append(stats.Completions, CompletionDetail{
			UserID:      c.UserID,
			Nickname:    u.Nickname,
			Gender:      gender,
			Region:      region,
			CityTier:    tier,
			School:      u.School,
			Major:       u.Major,
			AwaySeconds: c.AwaySeconds,
			CompletedAt: c.CreatedAt,
		})
	}
	stats.AvgAwaySeconds = float64(awaySum) / float64(len(completions))
	return stats, nil
}

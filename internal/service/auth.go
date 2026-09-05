package service

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	jwtauth "wenbang/internal/auth"
	"wenbang/internal/config"
	"wenbang/internal/model"
)

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUsernameTaken      = errors.New("用户名已被占用")
	ErrWeakInput          = errors.New("用户名和密码不能为空")
	ErrInvalidInviteCode  = errors.New("邀请链接无效或已失效")
	ErrInvalidProfile     = errors.New("请完善性别、南北方与城市线级")
)

const inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type AuthService struct {
	db *gorm.DB
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{db: db}
}

type AuthResult struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

type RegisterInput struct {
	Username   string
	Password   string
	Nickname   string
	School     string
	Major      string
	Gender     string
	Region     string
	CityTier   string
	InviteCode string
}

func (s *AuthService) Register(in RegisterInput) (*AuthResult, error) {
	username := strings.TrimSpace(in.Username)
	password := strings.TrimSpace(in.Password)
	school := strings.TrimSpace(in.School)
	major := strings.TrimSpace(in.Major)
	inviteCode := strings.ToUpper(strings.TrimSpace(in.InviteCode))
	gender := strings.TrimSpace(in.Gender)
	region := strings.TrimSpace(in.Region)
	cityTier := strings.TrimSpace(in.CityTier)
	if username == "" || password == "" {
		return nil, ErrWeakInput
	}
	if len(password) < 4 {
		return nil, errors.New("密码至少 4 位")
	}
	if !model.IsValidGender(gender) || !model.IsValidRegion(region) || !model.IsValidCityTier(cityTier) {
		return nil, ErrInvalidProfile
	}
	if major != "" && !validDiscipline(major) {
		return nil, errors.New("专业请选择学术型学科门类")
	}
	if school == "" {
		school = "中国人民大学"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	nickname := strings.TrimSpace(in.Nickname)
	if nickname == "" {
		nickname = username
	}

	var created *model.User
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrUsernameTaken
		}

		var inviter *model.User
		if inviteCode != "" {
			var u model.User
			if err := tx.Where("invite_code = ?", inviteCode).First(&u).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrInvalidInviteCode
				}
				return err
			}
			inviter = &u
		}

		code, err := s.uniqueInviteCode(tx)
		if err != nil {
			return err
		}

		pts := config.RegisterBonus()
		reward := config.InviteReward()
		var invitedBy *uint
		if inviter != nil {
			pts += reward
			id := inviter.ID
			invitedBy = &id
		}

		user := &model.User{
			Username:     username,
			PasswordHash: string(hash),
			Nickname:     nickname,
			School:       school,
			Major:        major,
			Gender:       gender,
			Region:       region,
			CityTier:     cityTier,
			InviteCode:   code,
			InvitedByID:  invitedBy,
			Points:       pts,
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		if inviter != nil && reward > 0 {
			if err := tx.Model(&model.User{}).
				Where("id = ?", inviter.ID).
				Update("points", gorm.Expr("points + ?", reward)).Error; err != nil {
				return err
			}
		}

		created = user
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.issue(created)
}

func (s *AuthService) Login(username, password string) (*AuthResult, error) {
	username = strings.TrimSpace(username)
	var user model.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if err := s.ensureInviteCode(&user); err != nil {
		return nil, err
	}
	return s.issue(&user)
}

func (s *AuthService) GetUser(id uint) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	if err := s.ensureInviteCode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

type UpdateProfileInput struct {
	Nickname string `json:"nickname"`
	School   string `json:"school"`
	Major    string `json:"major"`
	Gender   string `json:"gender"`
	Region   string `json:"region"`
	CityTier string `json:"city_tier"`
}

func (s *AuthService) UpdateProfile(userID uint, in UpdateProfileInput) (*model.User, error) {
	in.Nickname = strings.TrimSpace(in.Nickname)
	in.School = strings.TrimSpace(in.School)
	in.Major = strings.TrimSpace(in.Major)
	in.Gender = strings.TrimSpace(in.Gender)
	in.Region = strings.TrimSpace(in.Region)
	in.CityTier = strings.TrimSpace(in.CityTier)

	if in.Gender != "" && !model.IsValidGender(in.Gender) {
		return nil, errors.New("性别请选择：男 / 女 / 不便透露")
	}
	if in.Region != "" && !model.IsValidRegion(in.Region) {
		return nil, errors.New("南北方请选择：北方 / 南方")
	}
	if in.CityTier != "" && !model.IsValidCityTier(in.CityTier) {
		return nil, errors.New("城市线级不合法")
	}

	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	if in.Major != "" && !validDiscipline(in.Major) && in.Major != user.Major {
		return nil, errors.New("专业请选择学术型学科门类")
	}
	if in.School == "" {
		in.School = "中国人民大学"
	}
	if in.Nickname == "" {
		in.Nickname = user.Username
	}
	user.Nickname = in.Nickname
	user.School = in.School
	user.Major = in.Major
	if in.Gender != "" {
		user.Gender = in.Gender
	}
	if in.Region != "" {
		user.Region = in.Region
	}
	if in.CityTier != "" {
		user.CityTier = in.CityTier
	}
	if err := s.db.Save(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

var academicDisciplines = map[string]struct{}{
	"哲学": {}, "经济学": {}, "法学": {}, "教育学": {}, "文学": {}, "历史学": {},
	"理学": {}, "工学": {}, "农学": {}, "医学": {}, "军事学": {}, "管理学": {}, "艺术学": {},
}

func validDiscipline(major string) bool {
	_, ok := academicDisciplines[major]
	return ok
}

func (s *AuthService) ensureInviteCode(user *model.User) error {
	if strings.TrimSpace(user.InviteCode) != "" {
		return nil
	}
	code, err := s.uniqueInviteCode(s.db)
	if err != nil {
		return err
	}
	user.InviteCode = code
	return s.db.Model(user).Update("invite_code", code).Error
}

func (s *AuthService) uniqueInviteCode(tx *gorm.DB) (string, error) {
	for i := 0; i < 16; i++ {
		code, err := randomInviteCode(6)
		if err != nil {
			return "", err
		}
		var count int64
		if err := tx.Model(&model.User{}).Where("invite_code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", errors.New("生成邀请信息失败，请重试")
}

func randomInviteCode(n int) (string, error) {
	out := make([]byte, n)
	max := big.NewInt(int64(len(inviteCodeAlphabet)))
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = inviteCodeAlphabet[v.Int64()]
	}
	return string(out), nil
}

func (s *AuthService) issue(user *model.User) (*AuthResult, error) {
	token, err := jwtauth.GenerateToken(user.ID, user.Username, config.JWTSecret(), 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: token, User: user}, nil
}

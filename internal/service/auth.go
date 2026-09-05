package service

import (
	"errors"
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
	ErrInvalidDegreeTag   = errors.New("请选择有效的学位类别")
)

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

func (s *AuthService) Register(username, password, nickname, school, degreeTag string) (*AuthResult, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	degreeTag = strings.TrimSpace(degreeTag)
	if username == "" || password == "" {
		return nil, ErrWeakInput
	}
	if len(password) < 4 {
		return nil, errors.New("密码至少 4 位")
	}
	if !model.IsValidDegreeTag(degreeTag) {
		return nil, ErrInvalidDegreeTag
	}

	var count int64
	if err := s.db.Model(&model.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrUsernameTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if nickname == "" {
		nickname = username
	}

	user := &model.User{
		Username:     username,
		PasswordHash: string(hash),
		Nickname:     nickname,
		School:       school,
		DegreeTag:    degreeTag,
		Points:       config.RegisterBonus(),
	}
	if err := s.db.Create(user).Error; err != nil {
		return nil, err
	}
	return s.issue(user)
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
	return s.issue(&user)
}

func (s *AuthService) GetUser(id uint) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *AuthService) issue(user *model.User) (*AuthResult, error) {
	token, err := jwtauth.GenerateToken(user.ID, user.Username, config.JWTSecret(), 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: token, User: user}, nil
}

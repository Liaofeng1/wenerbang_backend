package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/smtp"
	"strings"
	"time"

	"gorm.io/gorm"

	"wenbang/internal/config"
	"wenbang/internal/model"
)

var (
	ErrEmailTaken        = errors.New("该邮箱已被注册")
	ErrEmailCodeInvalid  = errors.New("验证码错误或已过期")
	ErrEmailVerified     = errors.New("邮箱已验证通过")
	ErrEmailNotVerified  = errors.New("请先验证邮箱")
	ErrEmailCodeCooldown = errors.New("请稍后再试，验证码发送过于频繁")
)

type EmailService struct {
	db *gorm.DB
}

func NewEmailService(db *gorm.DB) *EmailService {
	return &EmailService{db: db}
}

// SendVerificationCode generates a 6-digit code and sends it to the email.
// In dev mode (no SMTP configured), it stores the code and returns it in a log message.
func (s *EmailService) SendVerificationCode(email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return errors.New("邮箱地址不能为空")
	}

	// Check if email is already registered
	var count int64
	if err := s.db.Model(&model.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrEmailTaken
	}

	// Check cooldown: don't allow resend within 60 seconds (check any record, regardless of verified status)
	var existing model.EmailVerification
	if err := s.db.Where("email = ?", email).First(&existing).Error; err == nil {
		if !existing.IsExpired() && time.Since(existing.CreatedAt).Seconds() < 60 {
			return ErrEmailCodeCooldown
		}
	}

	// Generate 6-digit code
	code, err := generateCode(6)
	if err != nil {
		return err
	}

	expireMinutes := config.EmailVerificationExpireMinutes()
	expiresAt := time.Now().Add(time.Duration(expireMinutes) * time.Minute)

	// Upsert verification record
	ev := &model.EmailVerification{
		Email:     email,
		Code:      code,
		ExpiresAt: expiresAt,
		Verified:  false,
	}

	// Delete old record if exists
	s.db.Where("email = ?", email).Delete(&model.EmailVerification{})

	if err := s.db.Create(ev).Error; err != nil {
		return err
	}

	// Send email if SMTP is configured
	if config.EmailEnabled() {
		if err := s.sendEmail(email, code, expireMinutes); err != nil {
			return err
		}
	} else {
		// Dev mode: print code to console
		fmt.Printf("\n========================================\n")
		fmt.Printf("  📧 [DEV MODE] Email verification code\n")
		fmt.Printf("  To: %s\n", email)
		fmt.Printf("  Code: %s\n", code)
		fmt.Printf("  Expires in: %d minutes\n", expireMinutes)
		fmt.Printf("========================================\n\n")
	}

	return nil
}

// VerifyCode checks the verification code and marks email as verified.
func (s *EmailService) VerifyCode(email, code string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	code = strings.TrimSpace(code)

	if email == "" || code == "" {
		return errors.New("邮箱和验证码不能为空")
	}

	var ev model.EmailVerification
	if err := s.db.Where("email = ?", email).First(&ev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEmailCodeInvalid
		}
		return err
	}

	if ev.Verified {
		return ErrEmailVerified
	}

	if !ev.IsValid(code) {
		return ErrEmailCodeInvalid
	}

	// Mark as verified
	if err := s.db.Model(&ev).Update("verified", true).Error; err != nil {
		return err
	}

	return nil
}

// CheckEmailVerified checks if the email has been verified.
// This is called during registration.
func (s *EmailService) CheckEmailVerified(email string) error {
	email = strings.TrimSpace(strings.ToLower(email))

	var ev model.EmailVerification
	if err := s.db.Where("email = ? AND verified = ?", email, true).First(&ev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEmailNotVerified
		}
		return err
	}
	return nil
}

// CleanExpiredVerifications removes expired verification records.
func (s *EmailService) CleanExpiredVerifications() {
	s.db.Where("expires_at < ?", time.Now()).Delete(&model.EmailVerification{})
}

func (s *EmailService) sendEmail(to, code string, expireMinutes int) error {
	host := config.SMTPHost()
	port := config.SMTPPort()
	user := config.SMTPUser()
	pass := config.SMTPPass()
	from := config.SMTPFrom()

	if from == "" {
		from = user
	}

	subject := "【问而帮】邮箱验证码"
	body := fmt.Sprintf(`尊敬的问而帮用户：

您好！

您的邮箱验证码为：<b>%s</b>

该验证码将在 %d 分钟内有效，请尽快完成验证。

如果这不是您本人的操作，请忽略此邮件。

问而帮团队`, code, expireMinutes)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	addr := fmt.Sprintf("%s:%d", host, port)
	auth := smtp.PlainAuth("", user, pass, host)

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

func generateCode(length int) (string, error) {
	code := make([]byte, length)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code[i] = byte('0') + byte(n.Int64())
	}
	return string(code), nil
}

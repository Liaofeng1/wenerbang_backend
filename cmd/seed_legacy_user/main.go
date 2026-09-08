package main

import (
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wenbang/internal/config"
	"wenbang/internal/model"
)

func main() {
	dsn := config.DatabaseDSN()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	// 生成密码哈希
	password := "test1234"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("generate hash: %v", err)
	}

	// 创建一个"老用户"（无邮箱）
	user := &model.User{
		Username:      "legacy_user",
		Email:         "",
		EmailVerified: false,
		PasswordHash:  string(hash),
		Nickname:      "老用户",
		School:        "中国人民大学",
		Major:         "经济学",
		Gender:        "男",
		Region:        "北方",
		CityTier:      "一线城市",
		InviteCode:    "TEST123",
		Points:        100,
	}

	// 先删除同名的旧数据（方便重复运行）
	db.Where("username = ?", user.Username).Delete(&model.User{})

	if err := db.Create(user).Error; err != nil {
		log.Fatalf("create user: %v", err)
	}

	fmt.Printf("✅ 测试老用户创建成功！\n")
	fmt.Printf("   用户名: legacy_user\n")
	fmt.Printf("   密码:   test1234\n")
	fmt.Printf("   邮箱:   (空 — 模拟老用户)\n")
	fmt.Printf("   邀请码: TEST123\n")
}

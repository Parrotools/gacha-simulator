package database

import (
	"log"

	"gacha-simulator/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDB initializes the SQLite database with the default file path "gacha.db".
func InitDB() error {
	return InitDBWithPath("gacha.db")
}

// InitDBWithPath initializes the SQLite database with the given file path.
func InitDBWithPath(path string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		log.Printf("Connection to database error: %v", err)
		return err
	}

	err = DB.AutoMigrate(
		&model.User{},
		&model.Character{},
		&model.UserCharacter{},
		&model.GachaRecord{},
		&model.DivinationRecord{},
	)
	if err != nil {
		log.Printf("migration failed: %v", err)
		return err
	}

	var adminCount int64
	DB.Model(&model.User{}).Where("role = ?", "admin").Count(&adminCount)
	if adminCount == 0 {
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		defaultAdmin := model.User{
			ID:       "admin",
			Nickname: "Overall admin",
			Password: string(hashedPwd),
			Role:     "admin",
		}
		DB.Create(&defaultAdmin)
	}

	return nil
}

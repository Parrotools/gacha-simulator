package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"gacha-simulator/internal/database"
	"gacha-simulator/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type EmailSender interface {
	SendCode(email, code string) error
}

type DefaultEmailSender struct{}

func (d *DefaultEmailSender) SendCode(email, code string) error {
	log.Printf("[EmailService] Sending verification code %s to %s", code, email)
	return nil
}

var CurrentEmailSender EmailSender = &DefaultEmailSender{}

type codeRecord struct {
	code      string
	expiresAt time.Time
}

var (
	codeMu    sync.RWMutex
	codeStore = make(map[string]codeRecord)
)

func StoreCode(email, code string) {
	codeMu.Lock()
	defer codeMu.Unlock()
	codeStore[email] = codeRecord{
		code:      code,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
}

func VerifyCode(email, code string) bool {
	codeMu.Lock()
	defer codeMu.Unlock()

	record, exists := codeStore[email]
	if !exists {
		return false
	}

	if time.Now().After(record.expiresAt) {
		delete(codeStore, email)
		return false
	}

	if record.code != code {
		return false
	}
	delete(codeStore, email)
	return true
}

func ClearCodeStore() {
	codeMu.Lock()
	defer codeMu.Unlock()
	codeStore = make(map[string]codeRecord)
}

type SendEmailCodeReq struct {
	Email string `json:"email" binding:"required,email"`
}

type EmailLoginReq struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
}

func SendEmailCodeHandler(c *gin.Context) {
	var req SendEmailCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format"})
		return
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate code"})
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	StoreCode(req.Email, code)
	if err := CurrentEmailSender.SendCode(req.Email, code); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send email code: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "verification code sent successfully",
	})
}

func EmailLoginHandler(c *gin.Context) {
	var req EmailLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
	}
	if !VerifyCode(req.Email, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired verification code"})
		return
	}

	var user model.User
	err := database.DB.Where("email = ?", req.Email).First(&user).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error: " + err.Error()})
		return
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		generatedID := uuid.New().String()[:8]
		emailPrefix := strings.Split(req.Email, "@")[0]
		hashedPwd, err := bcrypt.GenerateFromPassword([]byte(uuid.New().String()), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}
		user = model.User{
			ID:       generatedID,
			Nickname: emailPrefix,
			Password: string(hashedPwd),
			Email:    &req.Email,
			Role:     "user",
		}
		if err := database.DB.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user: " + err.Error()})
			return
		}
	}

	token, err := GenerateToken(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "login successfully",
		"token":   token,
		"user": gin.H{
			"id":       user.ID,
			"nickname": user.Nickname,
			"role":     user.Role,
			"bio":      user.Bio,
			"email":    user.Email,
		},
	})
}

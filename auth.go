package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type RegisterReq struct {
	Nickname string `json:"nickname" binding:"required"`
	Password string `json:"password" binding:"required"`
	Bio      string `json:"bio"`
}
type LoginReq struct {
	ID       string `json:"id" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type UpdateProfileReq struct {
	Nickname string `json:"nickname"`
	Bio      string `json:"bio"`
}
type SendEmailCodeReq struct {
	Email string `json:"email" binding:"required,email"`
}
type EmailLoginReq struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
}

func RegisterHandler(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wrong func"})
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode password"})
		return
	}
	generatedID := uuid.New().String()[:8]
	newUser := User{
		ID:       generatedID,
		Nickname: req.Nickname,
		Password: string(hashedPassword),
		Bio:      req.Bio,
		Role:     "user",
	}
	if err := DB.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save in db" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "registry successfully",
		"data": gin.H{
			"id":       newUser.ID,
			"nickname": newUser.Nickname,
		},
	})
}
func LoginHandler(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please enter pwd and account"})
		return
	}
	var user User
	if err := DB.Where("id = ? OR nickname = ?", req.ID, req.ID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "account not found"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wrong pwd"})
		return
	}
	token, err := GenerateToken(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token failed" + err.Error()})
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
		},
	})
}
func UpdateProfileHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	var req UpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wrong func"})
		return
	}
	var user User
	if err := DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid user"})
		return
	}
	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Bio != "" {
		user.Bio = req.Bio
	}
	DB.Save(&user)
	c.JSON(http.StatusOK, gin.H{
		"message": "updated succssfully",
		"data":    user,
	})

}
func LogoutHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "logged out!",
	})
}
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization failed"})
			c.Abort()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token invalid"})
			c.Abort()
			return
		}
		claims, err := ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token 无效或已过期"})
			c.Abort()
			return
		}
		c.Set("userID", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
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

	var user User
	err := DB.Where("email = ?", req.Email).First(&user).Error
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
		user = User{
			ID:       generatedID,
			Nickname: emailPrefix,
			Password: string(hashedPwd),
			Email:    &req.Email,
			Role:     "user",
		}
		if err := DB.Create(&user).Error; err != nil {
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

func GithubLoginHandler(c *gin.Context) {
	state := c.Query("state")
	if state == "" {
		state = uuid.New().String()[:8]
	}
	authURL := CurrentOAuthProvider.GetAuthURL(state)
	if c.Query("redirect") == "true" {
		c.Redirect(http.StatusTemporaryRedirect, authURL)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state,
	})
}

func GithubCallbackHandler(c *gin.Context) {
	var req struct {
		Code  string `json:"code" form:"code"`
		State string `json:"state" form:"state"`
	}
	_ = c.ShouldBindQuery(&req)
	if req.Code == "" {
		_ = c.ShouldBindJSON(&req)
	}
	if req.Code == "" {
		req.Code = c.Query("code")
	}
	if req.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "authorization code is required"})
		return
	}

	accessToken, err := CurrentOAuthProvider.ExchangeCode(req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to exchange code: " + err.Error()})
		return
	}

	profile, err := CurrentOAuthProvider.GetUserProfile(accessToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get github profile: " + err.Error()})
		return
	}

	ghIDStr := strconv.FormatInt(profile.ID, 10)
	var user User
	err = DB.Where("github_id = ?", ghIDStr).First(&user).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error: " + err.Error()})
		return
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if profile.Email != "" {
			var existingEmailUser User
			if err := DB.Where("email = ?", profile.Email).First(&existingEmailUser).Error; err == nil {
				existingEmailUser.GithubID = &ghIDStr
				if err := DB.Save(&existingEmailUser).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to link github account: " + err.Error()})
					return
				}
				user = existingEmailUser
			}
		}

		if user.ID == "" {
			generatedID := uuid.New().String()[:8]
			nickname := profile.Login
			if nickname == "" {
				nickname = profile.Name
			}
			if nickname == "" {
				nickname = "gh_" + ghIDStr
			}
			hashedPwd, err := bcrypt.GenerateFromPassword([]byte(uuid.New().String()), bcrypt.DefaultCost)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
				return
			}
			var emailPtr *string
			if profile.Email != "" {
				emailPtr = &profile.Email
			}
			user = User{
				ID:       generatedID,
				Nickname: nickname,
				Password: string(hashedPwd),
				GithubID: &ghIDStr,
				Email:    emailPtr,
				Role:     "user",
			}
			if err := DB.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user: " + err.Error()})
				return
			}
		}
	}

	token, err := GenerateToken(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token: " + err.Error()})
		return
	}

	if strings.Contains(c.GetHeader("Accept"), "text/html") {
		userMap := gin.H{
			"id":        user.ID,
			"nickname":  user.Nickname,
			"role":      user.Role,
			"bio":       user.Bio,
			"email":     user.Email,
			"github_id": user.GithubID,
		}
		userJSONBytes, _ := json.Marshal(userMap)
		htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>GitHub Login</title></head>
<body>
<p style="text-align:center;margin-top:20vh;font-family:sans-serif;color:#333;">GitHub 授权成功，正在跳转...</p>
<script>
    localStorage.setItem('gacha_token', '%s');
    localStorage.setItem('gacha_user', JSON.stringify(%s));
    window.location.href = '/';
</script>
</body>
</html>`, token, string(userJSONBytes))
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(htmlContent))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "github login successfully",
		"token":   token,
		"user": gin.H{
			"id":        user.ID,
			"nickname":  user.Nickname,
			"role":      user.Role,
			"bio":       user.Bio,
			"email":     user.Email,
			"github_id": user.GithubID,
		},
	})
}

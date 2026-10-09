package auth

import (
	"net/http"
	"strings"

	"gacha-simulator/internal/database"
	"gacha-simulator/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
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
	newUser := model.User{
		ID:       generatedID,
		Nickname: req.Nickname,
		Password: string(hashedPassword),
		Bio:      req.Bio,
		Role:     "user",
	}
	if err := database.DB.Create(&newUser).Error; err != nil {
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
	var user model.User
	if err := database.DB.Where("id = ? OR nickname = ?", req.ID, req.ID).First(&user).Error; err != nil {
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
	var user model.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid user"})
		return
	}
	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Bio != "" {
		user.Bio = req.Bio
	}
	database.DB.Save(&user)
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

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func GetMeHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")
	var user model.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"user_id":      user.ID,
			"nickname":     user.Nickname,
			"role":         user.Role,
			"bio":          user.Bio,
			"pity_s_count": user.PitySCount,
			"pity_a_count": user.PityACount,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user_id": userID, "role": role})
}

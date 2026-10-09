package gacha

import (
	"errors"
	"math/rand"
	"net/http"
	"time"

	"gacha-simulator/internal/database"
	"gacha-simulator/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FortuneOption struct {
	Sign         string `json:"sign"`
	Description  string `json:"description"`
	RewardAmount int    `json:"reward_amount"`
}

var astralFortunes = []FortuneOption{
	{
		Sign:         "大吉·星神注视",
		Description:  "星海泛起璀璨回音，今日十连必有金光闪烁！",
		RewardAmount: 160,
	},
	{
		Sign:         "中吉·跃迁顺风",
		Description:  "虚数航道畅通无阻，抽取限定角色概率处于巅峰态。",
		RewardAmount: 90,
	},
	{
		Sign:         "小吉·光锥流转",
		Description:  "忆质星尘随风萦绕，或将在奇数抽迎来惊喜。",
		RewardAmount: 60,
	},
	{
		Sign:         "平·虚数微澜",
		Description:  "命途平衡如常，稳扎稳打方见真章。",
		RewardAmount: 30,
	},
	{
		Sign:         "末吉·星核阻滞",
		Description:  "量子扰动略显剧烈，建议先洗手沐浴再开启跃迁。",
		RewardAmount: 10,
	},
}

// DivinationHandler handles POST /api/user/divination
func DivinationHandler(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录或登录已失效"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的用户标识"})
		return
	}

	today := time.Now().Format("2006-01-02")

	var existing model.DivinationRecord
	err := database.DB.Where("user_id = ? AND date = ?", userID, today).First(&existing).Error
	if err == nil {
		c.JSON(http.StatusOK, gin.H{
			"message":       "今日已完成占卜",
			"already_drawn": true,
			"data":          existing,
		})
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询占卜记录失败"})
		return
	}

	// Pick a random celestial fortune
	chosen := astralFortunes[rand.Intn(len(astralFortunes))]
	newRecord := model.DivinationRecord{
		UserID:       userID,
		Date:         today,
		Sign:         chosen.Sign,
		Description:  chosen.Description,
		RewardAmount: chosen.RewardAmount,
		CreatedAt:    time.Now(),
	}

	if err := database.DB.Create(&newRecord).Error; err != nil {
		// Concurrent creation race: fetch the created record
		var existing model.DivinationRecord
		if findErr := database.DB.Where("user_id = ? AND date = ?", userID, today).First(&existing).Error; findErr == nil {
			c.JSON(http.StatusOK, gin.H{
				"message":       "今日已完成占卜",
				"already_drawn": true,
				"data":          existing,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存占卜记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "占卜完成",
		"already_drawn": false,
		"data":          newRecord,
	})
}

// GetDivinationHandler handles GET /api/user/divination
func GetDivinationHandler(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录或登录已失效"})
		return
	}
	userID, ok := userIDVal.(string)
	if !ok || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的用户标识"})
		return
	}

	today := time.Now().Format("2006-01-02")

	var existing model.DivinationRecord
	err := database.DB.Where("user_id = ? AND date = ?", userID, today).First(&existing).Error
	if err == nil {
		c.JSON(http.StatusOK, gin.H{
			"message":       "今日已完成占卜",
			"already_drawn": true,
			"data":          existing,
		})
		return
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{
			"message":       "今日尚未占卜",
			"already_drawn": false,
			"data":          nil,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "查询占卜记录失败"})
}

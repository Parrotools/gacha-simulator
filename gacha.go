package main

import (
	"errors"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var drawMutex sync.Mutex

type DrawResult struct {
	Character   Character `json:"character"`
	IsFirstTime bool      `json:"is_first_time"`
	Rank        int       `json:"rank"`
	PityCountS  int       `json:"pity_count_s"`
}

func drawOnce(tx *gorm.DB, user *User) (*DrawResult, error) {
	user.PitySCount++
	user.PityACount++
	currentRateS := GlobalConfig.BaseRateS
	if user.PitySCount >= GlobalConfig.HardPityS {
		currentRateS = 1.0
	} else if user.PitySCount > GlobalConfig.SoftPityStart {
		extraPulls := user.PitySCount - GlobalConfig.SoftPityStart
		currentRateS += float64(extraPulls) * GlobalConfig.SoftPityInc
	}
	roll := rand.Float64()
	var hitRarity string
	if roll < currentRateS {
		hitRarity = "S"
	} else if roll < (currentRateS+GlobalConfig.BaseRateA) || user.PityACount >= GlobalConfig.HardPityA {
		hitRarity = "A"
	} else {
		hitRarity = "B"
	}
	var pickedChar Character
	if hitRarity == "S" {
		var poolSChars []Character
		if err := tx.Where("rarity = ? AND is_in_pool = ?", "S", true).Find(&poolSChars).Error; err != nil || len(poolSChars) == 0 {
			return nil, errors.New("no S currently")
		}
		var upChar *Character
		var otherChars []Character
		for i := range poolSChars {
			if poolSChars[i].IsUp {
				upChar = &poolSChars[i]
			} else {
				otherChars = append(otherChars, poolSChars[i])
			}
		}
		if upChar != nil && rand.Float64() < 0.5 {
			pickedChar = *upChar
		} else {
			if len(otherChars) > 0 {
				pickedChar = otherChars[rand.Intn(len(otherChars))]
			} else if upChar != nil {
				pickedChar = *upChar
			} else {
				pickedChar = poolSChars[rand.Intn(len(poolSChars))]
			}
		}

	} else {
		var poolChars []Character
		if err := tx.Where("rarity = ? AND is_in_pool = ?", hitRarity, true).Find(&poolChars).Error; err != nil || len(poolChars) == 0 {
			return nil, errors.New("卡池中暂无 " + hitRarity + " 档角色，请联系管理员补充")
		}
		pickedChar = poolChars[rand.Intn(len(poolChars))]
	}

	if pickedChar.ID == 0 {
		return nil, errors.New("invalid character picked: pool configuration error")
	}

	recordPityCount := user.PitySCount
	if hitRarity == "S" {
		user.PitySCount = 0
	} else if hitRarity == "A" {
		user.PityACount = 0
	}
	var userChar UserCharacter
	isFirstTime := false

	err := tx.Where("user_id = ? AND character_id = ?", user.ID, pickedChar.ID).First(&userChar).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		isFirstTime = true
		userChar = UserCharacter{
			UserID:      user.ID,
			CharacterID: pickedChar.ID,
			Rank:        0,
		}
		if err := tx.Create(&userChar).Error; err != nil {
			return nil, err
		}
	} else if err == nil {
		userChar.Rank++
		if err := tx.Save(&userChar).Error; err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	record := GachaRecord{
		UserID:        user.ID,
		CharacterID:   pickedChar.ID,
		CharacterName: pickedChar.Name,
		Rarity:        pickedChar.Rarity,
		IsFirstTime:   isFirstTime,
		PityCount:     recordPityCount,
		CreatedAt:     time.Now(),
	}
	if err := tx.Create(&record).Error; err != nil {
		return nil, err
	}

	return &DrawResult{
		Character:   pickedChar,
		IsFirstTime: isFirstTime,
		Rank:        userChar.Rank,
		PityCountS:  recordPityCount,
	}, nil
}

type DrawReq struct {
	Count int `json:"count" binding:"required"`
}

func DrawHandler(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req DrawReq
	if err := c.ShouldBindJSON(&req); err != nil || (req.Count != 1 && req.Count != 10) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误，抽卡次数 count 只能为 1 或 10"})
		return
	}

	drawMutex.Lock()
	defer drawMutex.Unlock()

	var results []DrawResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		for i := 0; i < req.Count; i++ {
			res, err := drawOnce(tx, &user)
			if err != nil {
				return err
			}
			results = append(results, *res)
		}
		return tx.Model(&user).Updates(map[string]interface{}{
			"pity_s_count": user.PitySCount,
			"pity_a_count": user.PityACount,
		}).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "抽卡失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "抽卡完成！",
		"count":   req.Count,
		"results": results,
	})
}
func GetUserInventoryHandler(c *gin.Context) {
	userID, _ := c.Get("userID")

	var characters []UserCharacter
	if err := DB.Preload("Character").Where("user_id = ?", userID).Find(&characters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询背包失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": len(characters),
		"data":  characters,
	})
}
func GetGachaHistoryHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 1
	} else if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	var records []GachaRecord
	var total int64

	DB.Model(&GachaRecord{}).Where("user_id = ?", userID).Count(&total)
	DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&records)

	c.JSON(http.StatusOK, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"data":      records,
	})
}

type SStatItem struct {
	CharacterName string `json:"character_name"`
	PullsTaken    int    `json:"pulls_taken"`
	IsUp          bool   `json:"is_up"`
	IsWon         string `json:"is_won"`
	PulledAt      string `json:"pulled_at"`
}

func GetGachaStatsHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	var sRecords []GachaRecord
	DB.Where("user_id = ? AND rarity = ?", userID, "S").Order("created_at ASC").Find(&sRecords)

	charMap := make(map[uint]Character)
	if len(sRecords) > 0 {
		var charIDs []uint
		seen := make(map[uint]bool)
		for _, rec := range sRecords {
			if !seen[rec.CharacterID] {
				seen[rec.CharacterID] = true
				charIDs = append(charIDs, rec.CharacterID)
			}
		}
		if len(charIDs) > 0 {
			var chars []Character
			DB.Where("id IN ?", charIDs).Find(&chars)
			for _, ch := range chars {
				charMap[ch.ID] = ch
			}
		}
	}

	var stats []SStatItem
	for _, rec := range sRecords {
		char := charMap[rec.CharacterID]

		status := "歪了！"
		if char.IsUp {
			status = "拿下 UP！未歪！"
		}

		stats = append(stats, SStatItem{
			CharacterName: rec.CharacterName,
			PullsTaken:    rec.PityCount,
			IsUp:          char.IsUp,
			IsWon:         status,
			PulledAt:      rec.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total_s_count": len(stats),
		"history":       stats,
	})
}
func ClearHistoryHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&GachaRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&UserCharacter{}).Error; err != nil {
			return err
		}
		return tx.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
			"pity_s_count": 0,
			"pity_a_count": 0,
		}).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "已成功抹除所有非酋记录与角色，保底计数已归零！重新出发吧！",
	})
}

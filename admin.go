package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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

type CreateCharacterReq struct {
	Name      string `json:"name" binding:"required"`
	Rarity    string `json:"rarity" binding:"required"` // "S", "A", "B"
	IsLimited bool   `json:"is_limited"`
}
type PushToPoolReq struct {
	CharacterID uint   `json:"character_id" binding:"required"`
	IsUp        bool   `json:"is_up"`
	Rarity      string `json:"rarity"`
	IsLimited   *bool  `json:"is_limited"`
}

var poolMutationMutex sync.Mutex
var errLimitedFlagRequiresS = errors.New("only S rarity characters can be limited")
var errUpFlagRequiresS = errors.New("only S rarity characters can be UP")

func normalizeCharacterDetails(name, rarity string) (string, string, error) {
	name = strings.TrimSpace(name)
	rarity = strings.TrimSpace(rarity)
	if name == "" {
		return "", "", errors.New("name is required")
	}
	if rarity != "S" && rarity != "A" && rarity != "B" {
		return "", "", errors.New("rarity must be S, A, or B")
	}
	return name, rarity, nil
}

func GetAdminCharactersHandler(c *gin.Context) {
	characters := make([]Character, 0)
	if err := DB.Order("id ASC").Find(&characters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load characters"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": characters})
}

func CreateCharacterHandler(c *gin.Context) {
	var req CreateCharacterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	name, rarity, err := normalizeCharacterDetails(req.Name, req.Rarity)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.IsLimited && rarity != "S" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only S rarity characters can be limited"})
		return
	}
	char := Character{
		Name:      name,
		Rarity:    rarity,
		IsLimited: req.IsLimited,
		IsInPool:  false,
	}
	if err := DB.Create(&char).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "character created successfully",
		"data":    char,
	})
}
func PushCharacterToPoolHandler(c *gin.Context) {
	var req PushToPoolReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wrong function"})
		return
	}
	if req.Rarity != "" {
		req.Rarity = strings.TrimSpace(req.Rarity)
		if req.Rarity != "S" && req.Rarity != "A" && req.Rarity != "B" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "rarity must be S, A, or B"})
			return
		}
	}

	poolMutationMutex.Lock()
	defer poolMutationMutex.Unlock()

	var char Character
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&char, req.CharacterID).Error; err != nil {
			return err
		}

		wasLimitedS := char.Rarity == "S" && char.IsLimited && char.IsInPool
		if req.Rarity != "" {
			char.Rarity = req.Rarity
		}
		if req.IsLimited != nil {
			char.IsLimited = *req.IsLimited
		}
		if char.IsLimited && char.Rarity != "S" {
			return errLimitedFlagRequiresS
		}
		if req.IsUp && char.Rarity != "S" {
			return errUpFlagRequiresS
		}

		now := time.Now()
		if !char.IsInPool || char.EnteredPoolAt == nil || (char.Rarity == "S" && char.IsLimited && !wasLimitedS) {
			char.EnteredPoolAt = &now
		}
		char.IsInPool = true
		char.IsUp = req.IsUp && char.Rarity == "S"
		if char.IsUp {
			if err := tx.Model(&Character{}).Where("id <> ? AND is_up = ?", char.ID, true).Update("is_up", false).Error; err != nil {
				return err
			}
		}
		if err := tx.Save(&char).Error; err != nil {
			return err
		}
		if char.Rarity == "S" && char.IsLimited {
			if err := enforceLimitedSCap(tx, GetCurrentPoolConfig().MaxLimitedS, char.ID, nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "unable to find the character"})
			return
		}
		if errors.Is(err, errLimitedFlagRequiresS) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, errUpFlagRequiresS) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to push into pool" + err.Error()})
		return
	}
	GlobalSSEBroker.Broadcast("POOL_UPDATE", fmt.Sprintf("角色【%s】已加入卡池！", char.Name))
	c.JSON(http.StatusOK, gin.H{
		"message": "character pushed into pool successfully",
		"data":    char,
	})
}
func GetPoolInfoHandler(c *gin.Context) {
	cfg := GetCurrentPoolConfig()
	var limitedS []Character
	DB.Where("rarity = ? AND is_limited = ? AND is_in_pool = ?", "S", true, true).Find(&limitedS)
	var upS Character
	hasUp := (DB.Where("rarity = ? AND is_up = ? AND is_in_pool = ?", "S", true, true).First(&upS).Error == nil)
	var standardChars []Character
	DB.Where("is_limited = ? AND is_in_pool = ?", false, true).Find(&standardChars)
	c.JSON(http.StatusOK, gin.H{
		"config": cfg,
		"banner": gin.H{
			"up_character": func() interface{} {
				if hasUp {
					return upS
				}
				return nil
			}(),
			"limited_s_character": limitedS,
			"standard_pool_count": len(standardChars),
		},
	})
}

type LoadPresetsReq struct {
	FilePath string `json:"file_path"`
}

type PresetCharacter struct {
	Name      string `json:"name"`
	Rarity    string `json:"rarity"`
	IsLimited bool   `json:"is_limited"`
	IsUp      bool   `json:"is_up"`
}

// enforceLimitedSCap evicts the oldest in-pool limited S characters until the
// configured cap is met. keepID is excluded when a pushed character must stay
// in the pool. Equal entry times are ordered by character ID.
func enforceLimitedSCap(tx *gorm.DB, maxLimitedS int, keepID uint, loadedChars *[]Character) error {
	var currentLimitedS []Character
	if err := tx.Where("rarity = ? AND is_limited = ? AND is_in_pool = ?", "S", true, true).
		Order("entered_pool_at ASC").Order("id ASC").Find(&currentLimitedS).Error; err != nil {
		return err
	}

	remaining := len(currentLimitedS)
	for i := range currentLimitedS {
		if remaining <= maxLimitedS {
			break
		}
		oldest := currentLimitedS[i]
		if oldest.ID == keepID {
			continue
		}
		if err := tx.Model(&oldest).Updates(map[string]interface{}{
			"is_in_pool": false,
			"is_up":      false,
		}).Error; err != nil {
			return err
		}
		remaining--
		if loadedChars != nil {
			for j := range *loadedChars {
				if (*loadedChars)[j].ID == oldest.ID {
					(*loadedChars)[j].IsInPool = false
					(*loadedChars)[j].IsUp = false
				}
			}
		}
	}
	return nil
}

func LoadPresetsHandler(c *gin.Context) {
	var req LoadPresetsReq
	if c.Request.Body != nil && c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request: " + err.Error()})
			return
		}
	}
	if req.FilePath == "" {
		req.FilePath = "presets/characters.json"
	}

	cleanPath := filepath.Clean(req.FilePath)
	slashPath := filepath.ToSlash(cleanPath)
	if slashPath != "presets" && !strings.HasPrefix(slashPath, "presets/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file path: must be located inside presets directory"})
		return
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read preset file: " + err.Error()})
		return
	}

	var presetList []PresetCharacter
	if err := json.Unmarshal(data, &presetList); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse preset json: " + err.Error()})
		return
	}
	if len(presetList) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "preset file is empty"})
		return
	}
	upCount := 0
	for i := range presetList {
		name, rarity, err := normalizeCharacterDetails(presetList[i].Name, presetList[i].Rarity)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character preset: " + err.Error()})
			return
		}
		presetList[i].Name = name
		presetList[i].Rarity = rarity
		if rarity != "S" && (presetList[i].IsLimited || presetList[i].IsUp) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "only S rarity characters can be limited or UP"})
			return
		}
		if presetList[i].IsUp {
			upCount++
			if upCount > 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "only one preset character can be UP"})
				return
			}
		}
	}

	var loadedChars []Character
	poolMutationMutex.Lock()
	defer poolMutationMutex.Unlock()
	err = DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for _, item := range presetList {
			var char Character
			err := tx.Where("name = ?", item.Name).First(&char).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				char = Character{
					Name:          item.Name,
					Rarity:        item.Rarity,
					IsLimited:     item.IsLimited,
					IsInPool:      true,
					IsUp:          item.IsUp,
					EnteredPoolAt: &now,
				}
				if item.IsUp {
					if err := tx.Model(&Character{}).Where("is_up = ?", true).Update("is_up", false).Error; err != nil {
						return err
					}
					for i := range loadedChars {
						loadedChars[i].IsUp = false
					}
				}
				if err := tx.Create(&char).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				wasLimitedS := char.Rarity == "S" && char.IsLimited && char.IsInPool
				wasInPool := char.IsInPool
				char.Rarity = item.Rarity
				char.IsLimited = item.IsLimited
				char.IsInPool = true
				if !wasInPool || char.EnteredPoolAt == nil || (item.Rarity == "S" && item.IsLimited && !wasLimitedS) {
					char.EnteredPoolAt = &now
				}
				char.IsUp = item.IsUp
				if item.IsUp {
					if err := tx.Model(&Character{}).Where("is_up = ?", true).Update("is_up", false).Error; err != nil {
						return err
					}
					for i := range loadedChars {
						loadedChars[i].IsUp = false
					}
				}
				if err := tx.Save(&char).Error; err != nil {
					return err
				}
			}
			loadedChars = append(loadedChars, char)
		}

		if err := enforceLimitedSCap(tx, GetCurrentPoolConfig().MaxLimitedS, 0, &loadedChars); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load presets: " + err.Error()})
		return
	}

	GlobalSSEBroker.Broadcast("POOL_UPDATE", fmt.Sprintf("已成功载入 %d 位预设角色到卡池！", len(loadedChars)))
	c.JSON(http.StatusOK, gin.H{
		"message": "presets loaded successfully",
		"count":   len(loadedChars),
		"data":    loadedChars,
	})
}

type UpdatePoolConfigReq struct {
	BaseRateS     *float64 `json:"base_rate_s"`
	BaseRateA     *float64 `json:"base_rate_a"`
	BaseRateB     *float64 `json:"base_rate_b"`
	SoftPityStart *int     `json:"soft_pity_start"`
	SoftPityInc   *float64 `json:"soft_pity_inc"`
	HardPityS     *int     `json:"hard_pity_s"`
	HardPityA     *int     `json:"hard_pity_a"`
	MaxLimitedS   *int     `json:"max_limited_s"`
}

func UpdatePoolConfigHandler(c *gin.Context) {
	var req UpdatePoolConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数解析失败: " + err.Error()})
		return
	}

	oldCfg := GetCurrentPoolConfig()
	newCfg := oldCfg

	if req.BaseRateS != nil {
		newCfg.BaseRateS = *req.BaseRateS
	}
	if req.BaseRateA != nil {
		newCfg.BaseRateA = *req.BaseRateA
	}
	if req.BaseRateB != nil {
		newCfg.BaseRateB = *req.BaseRateB
	} else if req.BaseRateS != nil || req.BaseRateA != nil {
		newCfg.BaseRateB = 1.0 - newCfg.BaseRateS - newCfg.BaseRateA
	}
	if req.SoftPityStart != nil {
		newCfg.SoftPityStart = *req.SoftPityStart
	}
	if req.SoftPityInc != nil {
		newCfg.SoftPityInc = *req.SoftPityInc
	}
	if req.HardPityS != nil {
		newCfg.HardPityS = *req.HardPityS
	}
	if req.HardPityA != nil {
		newCfg.HardPityA = *req.HardPityA
	}
	if req.MaxLimitedS != nil {
		newCfg.MaxLimitedS = *req.MaxLimitedS
	}

	if newCfg.BaseRateS < 0 || newCfg.BaseRateS > 1 ||
		newCfg.BaseRateA < 0 || newCfg.BaseRateA > 1 ||
		newCfg.BaseRateB < 0 || newCfg.BaseRateB > 1 ||
		newCfg.SoftPityStart < 0 ||
		newCfg.SoftPityInc < 0 || newCfg.SoftPityInc > 1 ||
		newCfg.HardPityS <= 0 ||
		newCfg.HardPityA <= 0 ||
		newCfg.MaxLimitedS <= 0 ||
		newCfg.SoftPityStart >= newCfg.HardPityS {
		c.JSON(http.StatusBadRequest, gin.H{"error": "配置参数校验失败：概率与保底数值不合法"})
		return
	}

	sum := newCfg.BaseRateS + newCfg.BaseRateA + newCfg.BaseRateB
	if math.Abs(sum-1.0) > 1e-6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "配置参数校验失败：基础概率总和(S+A+B)必须为1.0"})
		return
	}

	GlobalConfigAtomic.Store(&newCfg)
	BroadcastConfigToGRPC(&newCfg)
	GlobalSSEBroker.Broadcast("PROB_UPDATE", "卡池概率配置已更新！")

	c.JSON(http.StatusOK, gin.H{
		"message": "卡池配置更新成功",
		"config":  newCfg,
	})
}

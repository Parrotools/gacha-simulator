package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	CharacterID uint `json:"character_id" binding:"required"`
	IsUp        bool `json:"is_up"`
}

func CreateCharacterHandler(c *gin.Context) {
	var req CreateCharacterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	if req.Rarity != "S" && req.Rarity != "A" && req.Rarity != "B" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rarity must be S A or B"})
		return
	}
	char := Character{
		Name:      req.Name,
		Rarity:    req.Rarity,
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
	var char Character
	if err := DB.First(&char, req.CharacterID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unable to find the character"})
		return
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if char.Rarity == "S" {
			if char.IsLimited {
				var currentLimitedS []Character
				if err := tx.Where("rarity = ? AND is_limited = ? AND is_in_pool = ?", "S", true, true).Order("entered_pool_at ASC").Find(&currentLimitedS).Error; err != nil {
					return err
				}
				if len(currentLimitedS) >= GlobalConfig.MaxLimitedS {
					oldestChar := currentLimitedS[0]
					if err := tx.Model(&oldestChar).Updates(map[string]interface{}{
						"is_in_pool": false,
						"is_up":      false,
					}).Error; err != nil {
						return err
					}
				}
			}
			if req.IsUp {
				if err := tx.Model(&Character{}).Where("is_up = ?", true).Update("is_up", false).Error; err != nil {
					return err
				}
				char.IsUp = true
			}
		}
		char.IsInPool = true
		char.EnteredPoolAt = &now
		if err := tx.Save(&char).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to push into pool" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "character pushed into pool successfully",
		"data":    char,
	})
}
func GetPoolInfoHandler(c *gin.Context) {
	var limitedS []Character
	DB.Where("rarity = ? AND is_limited = ? AND is_in_pool = ?", "S", true, true).Find(&limitedS)
	var upS Character
	hasUp := (DB.Where("rarity = ? AND is_up = ? AND is_in_pool = ?", "S", true, true).First(&upS).Error == nil)
	var standardChars []Character
	DB.Where("is_limited = ? AND is_in_pool = ?", false, true).Find(&standardChars)
	c.JSON(http.StatusOK, gin.H{
		"config": GlobalConfig,
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

	var loadedChars []Character
	err = DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for _, item := range presetList {
			if item.Name == "" || (item.Rarity != "S" && item.Rarity != "A" && item.Rarity != "B") {
				return fmt.Errorf("invalid character preset: name=%s rarity=%s", item.Name, item.Rarity)
			}
			var char Character
			err := tx.Where("name = ?", item.Name).First(&char).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				char = Character{
					Name:          item.Name,
					Rarity:        item.Rarity,
					IsLimited:     item.IsLimited,
					IsInPool:      true,
					EnteredPoolAt: &now,
				}
				if item.Rarity == "S" && item.IsUp {
					if err := tx.Model(&Character{}).Where("is_up = ?", true).Update("is_up", false).Error; err != nil {
						return err
					}
					char.IsUp = true
					for i := range loadedChars {
						loadedChars[i].IsUp = false
					}
				} else {
					char.IsUp = false
				}
				if err := tx.Create(&char).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				char.Rarity = item.Rarity
				char.IsLimited = item.IsLimited
				char.IsInPool = true
				char.EnteredPoolAt = &now
				if item.Rarity == "S" && item.IsUp {
					if err := tx.Model(&Character{}).Where("is_up = ?", true).Update("is_up", false).Error; err != nil {
						return err
					}
					char.IsUp = true
					for i := range loadedChars {
						loadedChars[i].IsUp = false
					}
				} else {
					char.IsUp = false
				}
				if err := tx.Save(&char).Error; err != nil {
					return err
				}
			}
			loadedChars = append(loadedChars, char)
		}

		var currentLimitedS []Character
		if err := tx.Where("rarity = ? AND is_limited = ? AND is_in_pool = ?", "S", true, true).Order("entered_pool_at ASC").Find(&currentLimitedS).Error; err != nil {
			return err
		}
		if len(currentLimitedS) > GlobalConfig.MaxLimitedS {
			excess := len(currentLimitedS) - GlobalConfig.MaxLimitedS
			for i := 0; i < excess; i++ {
				oldest := currentLimitedS[i]
				if err := tx.Model(&oldest).Updates(map[string]interface{}{
					"is_in_pool": false,
					"is_up":      false,
				}).Error; err != nil {
					return err
				}
				for j := range loadedChars {
					if loadedChars[j].ID == oldest.ID {
						loadedChars[j].IsInPool = false
						loadedChars[j].IsUp = false
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load presets: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "presets loaded successfully",
		"count":   len(loadedChars),
		"data":    loadedChars,
	})
}

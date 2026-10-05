package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)
func AdminRequired() gin.HandlerFunc{
	return func(c *gin.Context){
		role,exists := c.Get("role")
		if !exists || role!="admin"{
			c.JSON(http.StatusForbidden, gin.H{"error":"permission denied"})
			c.Abort()
			return
		}
		c.Next()
	}
}
type CreateCharacterReq struct{
	Name      string `json:"name" binding:"required"`
	Rarity    string `json:"rarity" binding:"required"` // "S", "A", "B"
	IsLimited bool   `json:"is_limited"`
}
type PushToPoolReq struct{
	CharacterID uint `json:"character_id" binding:"required"`
	IsUp bool `json:"is_up"`
}
func CreateCharacterHandler(c *gin.Context){
	var req CreateCharacterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest,gin.H{"error":"bad request"})
		return 
	}
	if req.Rarity != "S"&& req.Rarity!="A"&&req.Rarity!="B"{
		c.JSON(http.StatusBadRequest,gin.H{"error":"rarity must be S A or B"})
		return 
	}
	char := Character{
		Name: req.Name,
		Rarity: req.Rarity,
		IsLimited: req.IsLimited,
		IsInPool: false,
	}
	if err := DB.Create(&char).Error; err!=nil{
		c.JSON(http.StatusInternalServerError, gin.H{"error":"internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":"character created successfully",
		"data":char,
	})
}
func PushCharacterToPoolHandler(c *gin.Context){
	var req PushToPoolReq
	if err := c.ShouldBindJSON(&req);err!=nil{
		c.JSON(http.StatusBadRequest, gin.H{"error":"wrong function"})
		return
	}
	var char Character
	if err:=DB.First(&char,req.CharacterID).Error;err!=nil{
		c.JSON(http.StatusNotFound, gin.H{"error":"unable to find the character"})
		return
	}
	err:=DB.Transaction(func(tx *gorm.DB)error{
		now:=time.Now()
		if char.Rarity == "S"{
			if char.IsLimited{
				var currentLimitedS []Character
				if err := tx.Where("rarity = ? AND is_limited = ? AND in_pool = ?", "S",true,true).Order("entered_pool_at ASC").Find(&currentLimitedS).Error;err!=nil{
					return err
				}
				if len(currentLimitedS)>=GlobalConfig.MaxLimitedS{
					oldestChar := currentLimitedS[0]
					if err := tx.Model(&oldestChar).Updates(map[string]interface{}{
						"in_pool":false,
						"is_up":false,
					}).Error;err!=nil{
						return err
					}
				}
			}
			if req.IsUp{
				if err:=tx.Model(&Character{}).Where("is_up = ?",true).Update("is_up", false).Error;err!=nil{
					return err
				}
				char.IsUp=true
			}
		}
		char.IsInPool = true
		char.EnteredPoolAt=&now
		if err:=tx.Save(&char).Error;err != nil{
			return err
		}
		return nil
	})
	if err!=nil{
		c.JSON(http.StatusInternalServerError,gin.H{"error":"failed to push into pool"+err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":"character pushed into pool successfully",
		"data":char,
	})
}
func GetPoolInfoHandler(c *gin.Context){
	var limitedS []Character
	DB.Where("rarity = ? AND is_limited = ? AND in_pool = ?", "S", true, true).Find(&limitedS)
	var upS Character
	hasUp := (DB.Where("rarity = ? AND is_up = ? AND in_pool = ?", "S", true, true).First(&upS).Error == nil)
	var standardChars []Character
	DB.Where("is_limited = ? AND in_pool = ?", false, true).Find(&standardChars)
	c.JSON(http.StatusOK, gin.H{
		"config":GlobalConfig,
		"banner":gin.H{
			"up_character":func ()interface{}  {
				if hasUp{
					return upS
				}
				return nil
			}(),
			"limited_s_character":limitedS,
			"standard_pool_count":len(standardChars),
		},
	})
}
package model

import (
	"sync/atomic"
	"time"
)

type User struct {
	ID         string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Nickname   string    `json:"nickname" gorm:"type:varchar(50);not null"`
	Password   string    `json:"-" gorm:"type:varchar(255);not null"`
	Bio        string    `json:"bio" gorm:"type:text"`
	Role       string    `json:"role" gorm:"type:varchar(20);default:'user'"`
	Email      *string   `json:"email,omitempty" gorm:"type:varchar(100);uniqueIndex"`
	GithubID   *string   `json:"github_id,omitempty" gorm:"type:varchar(100);uniqueIndex"`
	PitySCount int       `json:"pity_s_count" gorm:"default:0"`
	PityACount int       `json:"pity_a_count" gorm:"default:0"`
	CreatedAt  time.Time `json:"created_at"`
	UpdateAt   time.Time `json:"updated_at"`
}

type Character struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Name          string     `json:"name" gorm:"type:varchar(50);not null"`
	Rarity        string     `json:"rarity" gorm:"type:varchar(10);not null"`
	IsLimited     bool       `json:"is_limited" gorm:"default:false"`
	IsInPool      bool       `json:"in_pool" gorm:"column:is_in_pool;default:false"`
	IsUp          bool       `json:"is_up" gorm:"default:false"`
	EnteredPoolAt *time.Time `json:"entered_pool_at"`
}

type UserCharacter struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	UserID      string    `json:"user_id" gorm:"type:varchar(36);index"`
	CharacterID uint      `json:"character_id"`
	Character   Character `json:"character" gorm:"foreignKey:CharacterID"`
	Rank        int       `json:"rank" gorm:"default:0"`
}

type GachaRecord struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	UserID        string    `json:"user_id" gorm:"type:varchar(36);index"`
	CharacterID   uint      `json:"character_id"`
	CharacterName string    `json:"character_name"`
	Rarity        string    `json:"rarity"`
	IsFirstTime   bool      `json:"is_first_time"`
	PityCount     int       `json:"pity_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type DivinationRecord struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       string    `gorm:"uniqueIndex:idx_user_date;not null" json:"user_id"`
	Date         string    `gorm:"uniqueIndex:idx_user_date;not null" json:"date"` // format: "2006-01-02"
	Sign         string    `json:"sign"`                                           // e.g. "大吉·星神注视", "中吉·跃迁顺风", "平·虚数微澜", "末吉·星核阻滞"
	Description  string    `json:"description"`                                    // celestial fortune text
	RewardAmount int       `json:"reward_amount"`                                  // e.g. 60 or 100 Stellar Jade
	CreatedAt    time.Time `json:"created_at"`
}

type PoolConfig struct {
	BaseRateS     float64 `json:"base_rate_s"`
	BaseRateA     float64 `json:"base_rate_a"`
	BaseRateB     float64 `json:"base_rate_b"`
	SoftPityStart int     `json:"soft_pity_start"`
	SoftPityInc   float64 `json:"soft_pity_inc"`
	HardPityS     int     `json:"hard_pity_s"`
	HardPityA     int     `json:"hard_pity_a"`
	MaxLimitedS   int     `json:"max_limited_s"`
}

var CurrentConfigDefault = PoolConfig{
	BaseRateS:     0.008,
	BaseRateA:     0.080,
	BaseRateB:     0.912,
	SoftPityStart: 65,
	SoftPityInc:   0.050,
	HardPityS:     80,
	HardPityA:     10,
	MaxLimitedS:   3,
}

var GlobalConfig = CurrentConfigDefault
var GlobalConfigAtomic atomic.Pointer[PoolConfig]

func init() {
	GlobalConfigAtomic.Store(&CurrentConfigDefault)
}

func GetCurrentPoolConfig() PoolConfig {
	if cfg := GlobalConfigAtomic.Load(); cfg != nil {
		return *cfg
	}
	return CurrentConfigDefault
}

package main

import (
	"math"
	"math/rand"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SimulateReq struct {
	Pulls int `json:"pulls"`
}

type SimulateResp struct {
	TotalPulls     int        `json:"total_pulls"`
	SCount         int        `json:"s_count"`
	ACount         int        `json:"a_count"`
	BCount         int        `json:"b_count"`
	UpCount        int        `json:"up_count"`
	NonUpCount     int        `json:"non_up_count"`
	EmpiricalSRate float64    `json:"empirical_s_rate"`
	EmpiricalARate float64    `json:"empirical_a_rate"`
	AvgPullsPerS   float64    `json:"avg_pulls_per_s"`
	UpRate         float64    `json:"up_rate"`
	LuckScore      int        `json:"luck_score"`
	LuckLevel      string     `json:"luck_level"`
	CfgSnapshot    PoolConfig `json:"cfg_snapshot"`
}

// SimulateGachaHandler handles POST /api/gacha/simulate
func SimulateGachaHandler(c *gin.Context) {
	var req SimulateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to 1000 if empty or unparseable
		req.Pulls = 1000
	}

	if req.Pulls <= 0 {
		req.Pulls = 1000
	}
	if req.Pulls > 50000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "单次模拟抽卡次数不可超过 50,000"})
		return
	}

	cfg := GlobalConfigAtomic.Load()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "卡池配置不可用"})
		return
	}

	totalPulls := req.Pulls
	simPityS := 0
	simPityA := 0
	sCount := 0
	aCount := 0
	bCount := 0
	upCount := 0
	nonUpCount := 0
	sPullIntervals := make([]int, 0)

	for i := 0; i < totalPulls; i++ {
		simPityS++
		simPityA++

		currentRateS := cfg.BaseRateS
		if simPityS >= cfg.HardPityS {
			currentRateS = 1.0
		} else if simPityS > cfg.SoftPityStart {
			extraPulls := simPityS - cfg.SoftPityStart
			currentRateS += float64(extraPulls) * cfg.SoftPityInc
		}

		roll := rand.Float64()
		if roll < currentRateS {
			sCount++
			sPullIntervals = append(sPullIntervals, simPityS)
			simPityS = 0
			// 50% chance for UP character
			if rand.Float64() < 0.5 {
				upCount++
			} else {
				nonUpCount++
			}
		} else if roll < (currentRateS+cfg.BaseRateA) || simPityA >= cfg.HardPityA {
			aCount++
			simPityA = 0
		} else {
			bCount++
		}
	}

	empiricalS := float64(sCount) / float64(totalPulls) * 100.0
	empiricalA := float64(aCount) / float64(totalPulls) * 100.0

	var avgPullsPerS float64
	var upRate float64
	if sCount > 0 {
		avgPullsPerS = float64(totalPulls) / float64(sCount)
		upRate = float64(upCount) / float64(sCount) * 100.0
	}

	scoreCalc := 50.0 + (empiricalS-1.6)*15.0
	if sCount > 0 {
		scoreCalc += (upRate - 50.0) * 0.5
	}
	luckScore := int(math.Round(scoreCalc))
	if luckScore < 1 {
		luckScore = 1
	} else if luckScore > 100 {
		luckScore = 100
	}

	var luckLevel string
	switch {
	case luckScore >= 85:
		luckLevel = "天选欧皇"
	case luckScore >= 65:
		luckLevel = "欧气满满"
	case luckScore >= 45:
		luckLevel = "寻常修士"
	case luckScore >= 25:
		luckLevel = "非气微显"
	default:
		luckLevel = "终极非酋"
	}

	resp := SimulateResp{
		TotalPulls:     totalPulls,
		SCount:         sCount,
		ACount:         aCount,
		BCount:         bCount,
		UpCount:        upCount,
		NonUpCount:     nonUpCount,
		EmpiricalSRate: math.Round(empiricalS*100) / 100,
		EmpiricalARate: math.Round(empiricalA*100) / 100,
		AvgPullsPerS:   math.Round(avgPullsPerS*10) / 10,
		UpRate:         math.Round(upRate*100) / 100,
		LuckScore:      luckScore,
		LuckLevel:      luckLevel,
		CfgSnapshot:    *cfg,
	}

	c.JSON(http.StatusOK, resp)
}

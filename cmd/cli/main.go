package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[1;31m"
	colorGreen  = "\033[1;32m"
	colorYellow = "\033[1;33m" // Gold for S
	colorPurple = "\033[1;35m" // Purple for A
	colorCyan   = "\033[1;36m" // Cyan for B
	colorWhite  = "\033[1;37m"
	colorGray   = "\033[0;90m"
)

const tokenFile = ".gacha_cli_token"

type CLIConfig struct {
	ManagementServer string
	GameServer       string
	Token            string
	UserID           string
	Nickname         string
	PityS            int
	PityA            int
}

var cfg CLIConfig
var client = &http.Client{Timeout: 10 * time.Second}

func saveToken(token, userID, nickname string) {
	cfg.Token = token
	cfg.UserID = userID
	cfg.Nickname = nickname
	data := fmt.Sprintf("%s\n%s\n%s", token, userID, nickname)
	_ = os.WriteFile(tokenFile, []byte(data), 0600)
}

func loadToken() {
	data, err := os.ReadFile(tokenFile)
	if err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) >= 1 && strings.TrimSpace(lines[0]) != "" {
			cfg.Token = strings.TrimSpace(lines[0])
			if len(lines) >= 2 {
				cfg.UserID = strings.TrimSpace(lines[1])
			}
			if len(lines) >= 3 {
				cfg.Nickname = strings.TrimSpace(lines[2])
			}
		}
	}
}

func clearToken() {
	cfg.Token = ""
	cfg.UserID = ""
	cfg.Nickname = ""
	_ = os.Remove(tokenFile)
}

func sendJSONRequest(method, url string, reqBody interface{}, token string) (int, []byte, error) {
	var bodyReader io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return 0, nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return 0, nil, err
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	return resp.StatusCode, respBytes, err
}

func checkLoginStatus() bool {
	if cfg.Token == "" {
		return false
	}
	status, body, err := sendJSONRequest("GET", cfg.ManagementServer+"/api/user/me", nil, cfg.Token)
	if err != nil || status != http.StatusOK {
		clearToken()
		return false
	}
	var res struct {
		UserID   string `json:"user_id"`
		Nickname string `json:"nickname"`
		PityS    int    `json:"pity_s_count"`
		PityA    int    `json:"pity_a_count"`
	}
	if err := json.Unmarshal(body, &res); err == nil {
		cfg.UserID = res.UserID
		cfg.Nickname = res.Nickname
		cfg.PityS = res.PityS
		cfg.PityA = res.PityA
		return true
	}
	return false
}

func printBanner() {
	fmt.Printf("%s%s=================================================================%s\n", colorBold, colorYellow, colorReset)
	fmt.Printf("%s%s         ✦  ASTRAL GACHA SIMULATOR - TERMINAL CLI  ✦           %s\n", colorBold, colorYellow, colorReset)
	fmt.Printf("%s%s=================================================================%s\n", colorBold, colorYellow, colorReset)
	if cfg.Token != "" {
		fmt.Printf(" [用户]: %s%s (%s)%s | [S保底]: %s%d/80%s | [A保底]: %s%d/10%s\n",
			colorGreen, cfg.Nickname, cfg.UserID, colorReset,
			colorYellow, cfg.PityS, colorReset,
			colorPurple, cfg.PityA, colorReset)
	} else {
		fmt.Printf(" [状态]: %s未登录 (Guest)%s\n", colorRed, colorReset)
	}
	fmt.Printf(" [管理服务器]: %s%s%s | [游戏服务器]: %s%s%s\n",
		colorCyan, cfg.ManagementServer, colorReset,
		colorCyan, cfg.GameServer, colorReset)
	fmt.Printf("%s-----------------------------------------------------------------%s\n", colorGray, colorReset)
}

func printMenu() {
	fmt.Println(" 1.  登录 (Login)")
	fmt.Println(" 2.  注册 (Register)")
	fmt.Println(" 3.  查看卡池信息 (Pool Info)")
	fmt.Println(" 4.  单抽 (Draw 1)")
	fmt.Println(" 5.  十连抽 (Draw 10)")
	fmt.Println(" 6.  查看角色仓库 (Inventory)")
	fmt.Println(" 7.  抽卡战报统计 (Stats)")
	fmt.Println(" 8.  查看抽卡历史流水 (History)")
	fmt.Println(" 9.  清空抽卡历史与保底 (Clear History)")
	fmt.Println(" 10. 登出当前账号 (Logout)")
	fmt.Println(" 11. 星穹每日占卜 (Daily Divination)")
	fmt.Println(" 12. 蒙特卡洛抽卡测算 (Monte Carlo Sim)")
	fmt.Println(" 0.  退出程序 (Exit)")
	fmt.Printf("%s-----------------------------------------------------------------%s\n", colorGray, colorReset)
	fmt.Print("请选择操作 [0-12]: ")
}

func doLogin(reader *bufio.Scanner, explicitID, explicitPwd string) {
	id := explicitID
	pwd := explicitPwd
	if id == "" {
		fmt.Print("请输入用户 ID 或昵称: ")
		if !reader.Scan() {
			return
		}
		id = strings.TrimSpace(reader.Text())
	}
	if pwd == "" {
		fmt.Print("请输入密码: ")
		if !reader.Scan() {
			return
		}
		pwd = strings.TrimSpace(reader.Text())
	}

	payload := map[string]string{
		"id":       id,
		"password": pwd,
	}

	status, respBytes, err := sendJSONRequest("POST", cfg.ManagementServer+"/api/login", payload, "")
	if err != nil {
		fmt.Printf("%s[错误] 请求失败: %v%s\n", colorRed, err, colorReset)
		return
	}

	if status != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(respBytes, &errResp)
		msg := errResp.Error
		if msg == "" {
			msg = string(respBytes)
		}
		fmt.Printf("%s[错误] 登录失败 (%d): %s%s\n", colorRed, status, msg, colorReset)
		return
	}

	var res struct {
		Token string `json:"token"`
		User  struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		} `json:"user"`
	}
	if err := json.Unmarshal(respBytes, &res); err != nil {
		fmt.Printf("%s[错误] 解析响应失败: %v%s\n", colorRed, err, colorReset)
		return
	}

	saveToken(res.Token, res.User.ID, res.User.Nickname)
	checkLoginStatus()
	fmt.Printf("%s[成功] 登录成功！欢迎回来，%s (ID: %s)%s\n", colorGreen, res.User.Nickname, res.User.ID, colorReset)
}

func doRegister(reader *bufio.Scanner, explicitNick, explicitPwd, explicitBio string) {
	nick := explicitNick
	pwd := explicitPwd
	bio := explicitBio

	if nick == "" {
		fmt.Print("请输入昵称: ")
		if !reader.Scan() {
			return
		}
		nick = strings.TrimSpace(reader.Text())
	}
	if pwd == "" {
		fmt.Print("请输入密码: ")
		if !reader.Scan() {
			return
		}
		pwd = strings.TrimSpace(reader.Text())
	}
	if bio == "" {
		fmt.Print("请输入个人简介 (可留空): ")
		if reader.Scan() {
			bio = strings.TrimSpace(reader.Text())
		}
	}

	payload := map[string]string{
		"nickname": nick,
		"password": pwd,
		"bio":      bio,
	}

	status, respBytes, err := sendJSONRequest("POST", cfg.ManagementServer+"/api/register", payload, "")
	if err != nil {
		fmt.Printf("%s[错误] 请求失败: %v%s\n", colorRed, err, colorReset)
		return
	}

	if status != http.StatusOK {
		fmt.Printf("%s[错误] 注册失败 (%d): %s%s\n", colorRed, status, string(respBytes), colorReset)
		return
	}

	var res struct {
		Message string `json:"message"`
		Data    struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		} `json:"data"`
	}
	_ = json.Unmarshal(respBytes, &res)
	fmt.Printf("%s[成功] 注册成功！分配的唯一用户 ID 为: %s%s%s (请妥善保存)\n", colorGreen, colorBold, res.Data.ID, colorReset)

	// Automatically login
	fmt.Println("正在自动登录...")
	doLogin(reader, res.Data.ID, pwd)
}

func doPoolInfo() {
	status, respBytes, err := sendJSONRequest("GET", cfg.GameServer+"/api/pool/info", nil, "")
	if err != nil {
		fmt.Printf("%s[错误] 无法获取卡池信息: %v%s\n", colorRed, err, colorReset)
		return
	}
	if status != http.StatusOK {
		fmt.Printf("%s[错误] 获取卡池信息失败 (%d): %s%s\n", colorRed, status, string(respBytes), colorReset)
		return
	}

	var res struct {
		Config struct {
			BaseRateS     float64 `json:"base_rate_s"`
			BaseRateA     float64 `json:"base_rate_a"`
			BaseRateB     float64 `json:"base_rate_b"`
			SoftPityStart int     `json:"soft_pity_start"`
			SoftPityInc   float64 `json:"soft_pity_inc"`
			HardPityS     int     `json:"hard_pity_s"`
			HardPityA     int     `json:"hard_pity_a"`
			MaxLimitedS   int     `json:"max_limited_s"`
		} `json:"config"`
		Banner struct {
			UpCharacter *struct {
				ID        uint   `json:"id"`
				Name      string `json:"name"`
				Rarity    string `json:"rarity"`
				IsLimited bool   `json:"is_limited"`
				IsUp      bool   `json:"is_up"`
			} `json:"up_character"`
			LimitedSCharacter []struct {
				ID        uint   `json:"id"`
				Name      string `json:"name"`
				Rarity    string `json:"rarity"`
				IsLimited bool   `json:"is_limited"`
				IsUp      bool   `json:"is_up"`
			} `json:"limited_s_character"`
			StandardPoolCount int `json:"standard_pool_count"`
		} `json:"banner"`
	}

	if err := json.Unmarshal(respBytes, &res); err != nil {
		fmt.Printf("%s[错误] 解析卡池数据失败: %v%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("\n%s%s===== 当前卡池设定与概率 =====%s\n", colorBold, colorYellow, colorReset)
	fmt.Printf("• S 档基础概率: %s%.2f%%%s | 软保底: %d抽起 (每抽+%.1f%%) | 硬保底: %d抽\n",
		colorYellow, res.Config.BaseRateS*100, colorReset,
		res.Config.SoftPityStart, res.Config.SoftPityInc*100, res.Config.HardPityS)
	fmt.Printf("• A 档基础概率: %s%.2f%%%s | 硬保底: %d抽\n",
		colorPurple, res.Config.BaseRateA*100, colorReset, res.Config.HardPityA)
	fmt.Printf("• B 档基础概率: %s%.2f%%%s\n",
		colorCyan, res.Config.BaseRateB*100, colorReset)
	fmt.Printf("• 卡池限定 S 角色上限: %d 位\n", res.Config.MaxLimitedS)
	fmt.Println("-----------------------------------------------------------------")
	if res.Banner.UpCharacter != nil {
		fmt.Printf("★ 【本期 UP 角色】: %s%s%s (S档·限定UP 出现S时占50%%概率)\n",
			colorYellow, res.Banner.UpCharacter.Name, colorReset)
	} else {
		fmt.Printf("★ 【本期 UP 角色】: %s暂无 UP 角色%s\n", colorGray, colorReset)
	}
	fmt.Printf("★ 【当前池中限定 S 角色 (%d位)】: ", len(res.Banner.LimitedSCharacter))
	if len(res.Banner.LimitedSCharacter) == 0 {
		fmt.Println("无")
	} else {
		names := make([]string, 0, len(res.Banner.LimitedSCharacter))
		for _, c := range res.Banner.LimitedSCharacter {
			names = append(names, fmt.Sprintf("%s%s%s", colorYellow, c.Name, colorReset))
		}
		fmt.Println(strings.Join(names, ", "))
	}
	fmt.Printf("★ 【常驻角色/武器数量】: %d\n", res.Banner.StandardPoolCount)
	fmt.Printf("%s=================================================================%s\n\n", colorYellow, colorReset)
}

func doDraw(count int) {
	if cfg.Token == "" {
		fmt.Printf("%s[提示] 请先登录账号再进行抽卡！%s\n", colorRed, colorReset)
		return
	}

	payload := map[string]int{"count": count}
	status, respBytes, err := sendJSONRequest("POST", cfg.GameServer+"/api/gacha/draw", payload, cfg.Token)
	if err != nil {
		fmt.Printf("%s[错误] 抽卡请求失败: %v%s\n", colorRed, err, colorReset)
		return
	}
	if status != http.StatusOK {
		fmt.Printf("%s[错误] 抽卡失败 (%d): %s%s\n", colorRed, status, string(respBytes), colorReset)
		return
	}

	type DrawResult struct {
		Character struct {
			ID        uint   `json:"id"`
			Name      string `json:"name"`
			Rarity    string `json:"rarity"`
			IsLimited bool   `json:"is_limited"`
			IsUp      bool   `json:"is_up"`
		} `json:"character"`
		IsFirst bool `json:"is_first"`
	}

	var res struct {
		Count   int          `json:"count"`
		Results []DrawResult `json:"results"`
	}

	if err := json.Unmarshal(respBytes, &res); err != nil {
		fmt.Printf("%s[错误] 解析抽卡结果失败: %v%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("\n%s%s>>> 跃迁开始！抽卡结果如下 (%d 连抽) <<<%s\n", colorBold, colorYellow, count, colorReset)
	for i, r := range res.Results {
		var rarityBadge string
		switch r.Character.Rarity {
		case "S":
			badge := "★ S-RANK ★"
			if r.Character.IsUp {
				badge = "★ S-RANK (UP!) ★"
			}
			rarityBadge = fmt.Sprintf("%s%s%s", colorYellow, badge, colorReset)
		case "A":
			rarityBadge = fmt.Sprintf("%s[◆ A-RANK ◆]%s", colorPurple, colorReset)
		case "B":
			rarityBadge = fmt.Sprintf("%s[· B-RANK ·]%s", colorCyan, colorReset)
		}

		firstTag := ""
		if r.IsFirst {
			firstTag = fmt.Sprintf(" %s[NEW! 首次获得]%s", colorGreen, colorReset)
		}

		fmt.Printf(" [%02d] %-24s %-20s%s\n", i+1, rarityBadge, r.Character.Name, firstTag)
	}

	checkLoginStatus() // Refresh pity counters
	fmt.Printf("%s-----------------------------------------------------------------%s\n", colorGray, colorReset)
	fmt.Printf("当前保底计数: S保底 [%s%d/80%s] | A保底 [%s%d/10%s]\n\n",
		colorYellow, cfg.PityS, colorReset,
		colorPurple, cfg.PityA, colorReset)
}

func doInventory() {
	if cfg.Token == "" {
		fmt.Printf("%s[提示] 请先登录账号！%s\n", colorRed, colorReset)
		return
	}

	status, respBytes, err := sendJSONRequest("GET", cfg.GameServer+"/api/gacha/inventory", nil, cfg.Token)
	if err != nil {
		fmt.Printf("%s[错误] 请求失败: %v%s\n", colorRed, err, colorReset)
		return
	}
	if status != http.StatusOK {
		fmt.Printf("%s[错误] 获取仓库失败 (%d): %s%s\n", colorRed, status, string(respBytes), colorReset)
		return
	}

	type UserCharItem struct {
		CharacterID uint `json:"character_id"`
		Count       int  `json:"count"`
		Character   struct {
			Name      string `json:"name"`
			Rarity    string `json:"rarity"`
			IsLimited bool   `json:"is_limited"`
		} `json:"character"`
	}

	var res struct {
		Total int            `json:"total"`
		Data  []UserCharItem `json:"data"`
	}

	if err := json.Unmarshal(respBytes, &res); err != nil {
		fmt.Printf("%s[错误] 解析仓库失败: %v%s\n", colorRed, err, colorReset)
		return
	}

	if res.Total == 0 {
		fmt.Println("\n[背包状态] 仓库空空如也，快去抽卡吧！")
		return
	}

	// Sort: S > A > B, then by Count desc
	rarityWeight := map[string]int{"S": 3, "A": 2, "B": 1}
	sort.Slice(res.Data, func(i, j int) bool {
		w1 := rarityWeight[res.Data[i].Character.Rarity]
		w2 := rarityWeight[res.Data[j].Character.Rarity]
		if w1 != w2 {
			return w1 > w2
		}
		return res.Data[i].Count > res.Data[j].Count
	})

	fmt.Printf("\n%s%s===== 角色仓库清单 (已拥有种类: %d) =====%s\n", colorBold, colorGreen, res.Total, colorReset)
	fmt.Printf("%-6s | %-6s | %-24s | %-8s | %-12s\n", "序号", "品质", "角色/物品名称", "持有数量", "等阶(潜能)")
	fmt.Println("-----------------------------------------------------------------")
	for i, uc := range res.Data {
		var rarityStr string
		switch uc.Character.Rarity {
		case "S":
			rarityStr = fmt.Sprintf("%s S %s", colorYellow, colorReset)
		case "A":
			rarityStr = fmt.Sprintf("%s A %s", colorPurple, colorReset)
		default:
			rarityStr = fmt.Sprintf("%s B %s", colorCyan, colorReset)
		}
		potential := uc.Count - 1
		fmt.Printf("%-4d   | %s | %-20s | %-6d   | %s%d 阶%s\n",
			i+1, rarityStr, uc.Character.Name, uc.Count, colorBold, potential, colorReset)
	}
	fmt.Println("-----------------------------------------------------------------")
	fmt.Println()
}

func doStats() {
	if cfg.Token == "" {
		fmt.Printf("%s[提示] 请先登录账号！%s\n", colorRed, colorReset)
		return
	}

	status, respBytes, err := sendJSONRequest("GET", cfg.GameServer+"/api/gacha/stats", nil, cfg.Token)
	if err != nil {
		fmt.Printf("%s[错误] 请求失败: %v%s\n", colorRed, err, colorReset)
		return
	}
	if status != http.StatusOK {
		fmt.Printf("%s[错误] 获取统计失败 (%d): %s%s\n", colorRed, status, string(respBytes), colorReset)
		return
	}

	type SStatItem struct {
		CharacterName string `json:"character_name"`
		DrawCount     int    `json:"draw_count"`
		IsUp          bool   `json:"is_up"`
		CreatedAt     string `json:"created_at"`
	}

	var res struct {
		TotalSCount int         `json:"total_s_count"`
		History     []SStatItem `json:"history"`
	}

	if err := json.Unmarshal(respBytes, &res); err != nil {
		fmt.Printf("%s[错误] 解析统计失败: %v%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("\n%s%s===== 抽卡战报分析 =====%s\n", colorBold, colorYellow, colorReset)
	fmt.Printf("累计获得 S 级角色: %s%d%s 位\n", colorYellow, res.TotalSCount, colorReset)
	if res.TotalSCount == 0 {
		fmt.Println("目前暂无 S 级出金记录，继续加油！")
		fmt.Println()
		return
	}

	totalPulls := 0
	for _, item := range res.History {
		totalPulls += item.DrawCount
	}
	avgPulls := float64(totalPulls) / float64(res.TotalSCount)
	fmt.Printf("平均出金抽数: %s%.1f 抽%s\n", colorGreen, avgPulls, colorReset)
	fmt.Println("-----------------------------------------------------------------")
	fmt.Printf("%-6s | %-22s | %-8s | %-12s\n", "出金次序", "获得角色", "消耗抽数", "歪/UP判定")
	fmt.Println("-----------------------------------------------------------------")

	for i, item := range res.History {
		upTag := fmt.Sprintf("%s【本期UP】%s", colorGreen, colorReset)
		if !item.IsUp {
			upTag = fmt.Sprintf("%s【常驻歪】%s", colorRed, colorReset)
		}
		fmt.Printf("第 %-4d 位 | %s%-20s%s | %-6d 抽 | %s\n",
			i+1, colorYellow, item.CharacterName, colorReset, item.DrawCount, upTag)
	}
	fmt.Println("-----------------------------------------------------------------")
	fmt.Println()
}

func doHistory() {
	if cfg.Token == "" {
		fmt.Printf("%s[提示] 请先登录账号！%s\n", colorRed, colorReset)
		return
	}

	status, respBytes, err := sendJSONRequest("GET", cfg.GameServer+"/api/gacha/history?page=1&page_size=20", nil, cfg.Token)
	if err != nil {
		fmt.Printf("%s[错误] 请求失败: %v%s\n", colorRed, err, colorReset)
		return
	}
	if status != http.StatusOK {
		fmt.Printf("%s[错误] 获取历史流水失败 (%d): %s%s\n", colorRed, status, string(respBytes), colorReset)
		return
	}

	type HistItem struct {
		ID        uint   `json:"id"`
		IsFirst   bool   `json:"is_first"`
		CreatedAt string `json:"created_at"`
		Character struct {
			Name   string `json:"name"`
			Rarity string `json:"rarity"`
		} `json:"character"`
	}

	var res struct {
		Total    int64      `json:"total"`
		Page     int        `json:"page"`
		PageSize int        `json:"page_size"`
		Data     []HistItem `json:"data"`
	}

	if err := json.Unmarshal(respBytes, &res); err != nil {
		fmt.Printf("%s[错误] 解析历史失败: %v%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("\n%s%s===== 抽卡历史流水 (总计 %d 条，展示最新 %d 条) =====%s\n",
		colorBold, colorCyan, res.Total, len(res.Data), colorReset)
	if len(res.Data) == 0 {
		fmt.Println("暂无抽卡记录。")
		fmt.Println()
		return
	}

	fmt.Printf("%-6s | %-6s | %-22s | %-12s | %-20s\n", "流水号", "品质", "获得角色/物品", "首次获得", "时间")
	fmt.Println("-----------------------------------------------------------------")
	for _, item := range res.Data {
		var rarityStr string
		switch item.Character.Rarity {
		case "S":
			rarityStr = fmt.Sprintf("%s S %s", colorYellow, colorReset)
		case "A":
			rarityStr = fmt.Sprintf("%s A %s", colorPurple, colorReset)
		default:
			rarityStr = fmt.Sprintf("%s B %s", colorCyan, colorReset)
		}

		firstStr := "否"
		if item.IsFirst {
			firstStr = fmt.Sprintf("%s是 (NEW)%s", colorGreen, colorReset)
		}

		tStr := item.CreatedAt
		if len(tStr) >= 19 {
			tStr = tStr[:19]
		}
		fmt.Printf("%-6d | %s | %-18s | %-10s | %s\n",
			item.ID, rarityStr, item.Character.Name, firstStr, tStr)
	}
	fmt.Println("-----------------------------------------------------------------")
	fmt.Println()
}

func doClearHistory(reader *bufio.Scanner) {
	if cfg.Token == "" {
		fmt.Printf("%s[提示] 请先登录账号！%s\n", colorRed, colorReset)
		return
	}

	fmt.Printf("%s[警告] 此操作将永久删除您所有的抽卡流水记录、拥有的角色背包，并将保底重置为0！%s\n", colorRed, colorReset)
	fmt.Print("确认清空吗？请输入 'y' 确认: ")
	if !reader.Scan() {
		return
	}
	confirm := strings.TrimSpace(reader.Text())
	if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
		fmt.Println("已取消操作。")
		return
	}

	status, respBytes, err := sendJSONRequest("DELETE", cfg.GameServer+"/api/gacha/history", nil, cfg.Token)
	if err != nil {
		fmt.Printf("%s[错误] 请求失败: %v%s\n", colorRed, err, colorReset)
		return
	}
	if status != http.StatusOK {
		fmt.Printf("%s[错误] 清空失败 (%d): %s%s\n", colorRed, status, string(respBytes), colorReset)
		return
	}

	checkLoginStatus()
	fmt.Printf("%s[成功] 抽卡流水已成功清空，仓库已归零，保底已重置为0！%s\n\n", colorGreen, colorReset)
}

func doDivination() {
	if cfg.Token == "" {
		fmt.Printf("%s[提示] 请先登录账号！%s\n", colorRed, colorReset)
		return
	}

	status, respBytes, err := sendJSONRequest("POST", cfg.ManagementServer+"/api/user/divination", nil, cfg.Token)
	if err != nil {
		fmt.Printf("%s[错误] 请求失败: %v%s\n", colorRed, err, colorReset)
		return
	}
	if status != http.StatusOK {
		fmt.Printf("%s[错误] 占卜失败 (%d): %s%s\n", colorRed, status, string(respBytes), colorReset)
		return
	}

	var res struct {
		Message      string `json:"message"`
		AlreadyDrawn bool   `json:"already_drawn"`
		Data         struct {
			Sign         string `json:"sign"`
			Description  string `json:"description"`
			RewardAmount int    `json:"reward_amount"`
			Date         string `json:"date"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBytes, &res); err != nil {
		fmt.Printf("%s[错误] 解析结果失败: %v%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("\n%s%s=================================================================%s\n", colorBold, colorYellow, colorReset)
	fmt.Printf("%s%s         ✦  星穹每日星占 · ASTRAL DAILY DIVINATION  ✦         %s\n", colorBold, colorYellow, colorReset)
	fmt.Printf("%s%s=================================================================%s\n", colorBold, colorYellow, colorReset)
	statusTag := fmt.Sprintf("%s【今日初占】%s", colorGreen, colorReset)
	if res.AlreadyDrawn {
		statusTag = fmt.Sprintf("%s【今日已签】%s", colorCyan, colorReset)
	}
	fmt.Printf(" [状态]: %s  |  [日期]: %s%s%s\n", statusTag, colorWhite, res.Data.Date, colorReset)
	fmt.Println("-----------------------------------------------------------------")
	fmt.Printf(" 占卜星象: %s%s%s\n", colorYellow, colorBold, res.Data.Sign)
	fmt.Printf(" 命途神谕: %s%s%s\n", colorCyan, res.Data.Description, colorReset)
	fmt.Printf(" 星穹馈赠: %s+%d 星琼 (Stellar Jade)%s\n", colorGreen, res.Data.RewardAmount, colorReset)
	fmt.Printf("%s=================================================================%s\n\n", colorYellow, colorReset)
}

func doSimulate(reader *bufio.Scanner, explicitPulls int) {
	if cfg.Token == "" {
		fmt.Printf("%s[提示] 请先登录账号！%s\n", colorRed, colorReset)
		return
	}

	pulls := explicitPulls
	if pulls <= 0 {
		fmt.Print("请输入拟测算的跃迁抽数 [默认 1000, 范围 1-50000]: ")
		if reader != nil && reader.Scan() {
			input := strings.TrimSpace(reader.Text())
			if input != "" {
				if val, err := strconv.Atoi(input); err == nil && val > 0 {
					pulls = val
				} else {
					fmt.Printf("%s输入无效，将采用默认 1000 抽进行测算%s\n", colorYellow, colorReset)
					pulls = 1000
				}
			} else {
				pulls = 1000
			}
		} else {
			pulls = 1000
		}
	}

	reqBody := map[string]int{"pulls": pulls}
	status, respBytes, err := sendJSONRequest("POST", cfg.GameServer+"/api/gacha/simulate", reqBody, cfg.Token)
	if err != nil {
		fmt.Printf("%s[错误] 请求失败: %v%s\n", colorRed, err, colorReset)
		return
	}
	if status != http.StatusOK {
		fmt.Printf("%s[错误] 测算失败 (%d): %s%s\n", colorRed, status, string(respBytes), colorReset)
		return
	}

	var res struct {
		TotalPulls     int     `json:"total_pulls"`
		SCount         int     `json:"s_count"`
		ACount         int     `json:"a_count"`
		BCount         int     `json:"b_count"`
		UpCount        int     `json:"up_count"`
		NonUpCount     int     `json:"non_up_count"`
		EmpiricalSRate float64 `json:"empirical_s_rate"`
		EmpiricalARate float64 `json:"empirical_a_rate"`
		AvgPullsPerS   float64 `json:"avg_pulls_per_s"`
		UpRate         float64 `json:"up_rate"`
		LuckScore      int     `json:"luck_score"`
		LuckLevel      string  `json:"luck_level"`
	}

	if err := json.Unmarshal(respBytes, &res); err != nil {
		fmt.Printf("%s[错误] 解析测算结果失败: %v%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("\n%s%s=================================================================%s\n", colorBold, colorYellow, colorReset)
	fmt.Printf("%s%s       ✦ 蒙特卡洛星轨概率测算 (MONTE CARLO SIMULATION) ✦        %s\n", colorBold, colorYellow, colorReset)
	fmt.Printf("%s%s=================================================================%s\n", colorBold, colorYellow, colorReset)
	fmt.Printf(" 模拟跃迁总抽数: %s%d 抽%s\n", colorBold, res.TotalPulls, colorReset)
	fmt.Println("-----------------------------------------------------------------")
	fmt.Println(" 【品质掉落统计】:")
	fmt.Printf("   ✦ S级金光:  %s%4d 次%s (综合出率: %s%.2f%%%s) [UP: %s%d%s | 常驻: %s%d%s]\n",
		colorYellow, res.SCount, colorReset,
		colorYellow, res.EmpiricalSRate, colorReset,
		colorGreen, res.UpCount, colorReset,
		colorRed, res.NonUpCount, colorReset)
	fmt.Printf("   ★ A级紫光:  %s%4d 次%s (综合出率: %s%.2f%%%s)\n",
		colorPurple, res.ACount, colorReset,
		colorPurple, res.EmpiricalARate, colorReset)
	bRate := 100.0 - res.EmpiricalSRate - res.EmpiricalARate
	fmt.Printf("   ◆ B级蓝光:  %s%4d 次%s (综合出率: %s%.2f%%%s)\n",
		colorCyan, res.BCount, colorReset,
		colorCyan, bRate, colorReset)
	fmt.Println("-----------------------------------------------------------------")
	fmt.Println(" 【深度概率度量】:")
	if res.SCount > 0 {
		fmt.Printf("   平均出金间隔: 每 %s%.1f%s 抽获得一次 S 级\n", colorYellow, res.AvgPullsPerS, colorReset)
		fmt.Printf("   UP 角色不歪率: %s%.2f%%%s (理论基准: 50.00%%)\n", colorGreen, res.UpRate, colorReset)
	} else {
		fmt.Printf("   平均出金间隔: %s未能获得 S 级%s\n", colorRed, colorReset)
		fmt.Printf("   UP 角色不歪率: N/A\n")
	}
	scoreColor := colorGreen
	if res.LuckScore >= 75 {
		scoreColor = colorYellow
	} else if res.LuckScore < 45 {
		scoreColor = colorRed
	}
	fmt.Printf("   欧皇指数评分: %s%d / 100%s  %s【%s】%s\n",
		scoreColor, res.LuckScore, colorReset,
		scoreColor, res.LuckLevel, colorReset)
	fmt.Printf("%s=================================================================%s\n\n", colorYellow, colorReset)
}

func main() {
	serverFlag := flag.String("server", "http://localhost:8080", "Management Server URL")
	gameServerFlag := flag.String("game-server", "", "Game Server URL (defaults to --server if empty)")
	cmdFlag := flag.String("cmd", "", "Non-interactive command: login, register, pool, draw1, draw10, inventory, stats, history, clear, divination, sim")
	pullsFlag := flag.Int("pulls", 1000, "Number of pulls for Monte Carlo simulation")
	idFlag := flag.String("id", "", "User ID for direct login/register")
	pwdFlag := flag.String("password", "", "Password for direct login/register")
	nickFlag := flag.String("nickname", "", "Nickname for direct register")
	bioFlag := flag.String("bio", "", "Bio for direct register")
	flag.Parse()

	cfg.ManagementServer = strings.TrimRight(*serverFlag, "/")
	if *gameServerFlag != "" {
		cfg.GameServer = strings.TrimRight(*gameServerFlag, "/")
	} else {
		cfg.GameServer = cfg.ManagementServer
	}

	loadToken()
	checkLoginStatus()

	// Direct non-interactive execution mode
	if *cmdFlag != "" {
		scanner := bufio.NewScanner(os.Stdin)
		switch *cmdFlag {
		case "login":
			doLogin(scanner, *idFlag, *pwdFlag)
		case "register":
			doRegister(scanner, *nickFlag, *pwdFlag, *bioFlag)
		case "pool", "info":
			doPoolInfo()
		case "draw1", "draw":
			doDraw(1)
		case "draw10":
			doDraw(10)
		case "inventory", "inv":
			doInventory()
		case "stats":
			doStats()
		case "history":
			doHistory()
		case "clear":
			doClearHistory(scanner)
		case "divination", "div":
			doDivination()
		case "sim", "simulate":
			doSimulate(scanner, *pullsFlag)
		default:
			fmt.Printf("未知命令: %s. 支持的命令: login, register, pool, draw1, draw10, inventory, stats, history, clear, divination, sim\n", *cmdFlag)
			os.Exit(1)
		}
		return
	}

	// Interactive REPL Mode
	scanner := bufio.NewScanner(os.Stdin)
	for {
		printBanner()
		printMenu()

		if !scanner.Scan() {
			break
		}
		choice := strings.TrimSpace(scanner.Text())

		switch choice {
		case "1":
			doLogin(scanner, "", "")
		case "2":
			doRegister(scanner, "", "", "")
		case "3":
			doPoolInfo()
		case "4":
			doDraw(1)
		case "5":
			doDraw(10)
		case "6":
			doInventory()
		case "7":
			doStats()
		case "8":
			doHistory()
		case "9":
			doClearHistory(scanner)
		case "10":
			clearToken()
			fmt.Printf("%s[提示] 已登出当前账号并清除本地凭证。%s\n\n", colorYellow, colorReset)
		case "11":
			doDivination()
		case "12":
			doSimulate(scanner, 0)
		case "0", "exit", "quit":
			fmt.Println("\n感谢使用 Astral 抽卡模拟器，祝您出金不歪，再见！")
			return
		default:
			fmt.Printf("%s无效输入，请输入 0-12 之间的数字。%s\n\n", colorRed, colorReset)
		}

		fmt.Print("按回车键继续...")
		_ = scanner.Scan()
		fmt.Println()
	}
}

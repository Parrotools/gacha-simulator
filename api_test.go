package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	proto "gacha-simulator/proto"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = DB.AutoMigrate(
		&User{},
		&Character{},
		&UserCharacter{},
		&GachaRecord{},
	)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	defaultAdmin := User{
		ID:       "admin",
		Nickname: "Overall admin",
		Password: string(hashedPwd),
		Role:     "admin",
	}
	if err := DB.Create(&defaultAdmin).Error; err != nil {
		t.Fatalf("Failed to create default admin: %v", err)
	}
}

func performRequest(r http.Handler, method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	var reqBody *bytes.Reader
	if body != nil {
		jsonBytes, _ := json.Marshal(body)
		reqBody = bytes.NewReader(jsonBytes)
	} else {
		reqBody = bytes.NewReader([]byte{})
	}
	req, _ := http.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthFlow(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()
	regReq := map[string]string{
		"nickname": "gacha_fan",
		"password": "secretpassword",
		"bio":      "Testing gacha simulator",
	}
	w := performRequest(router, "POST", "/api/register", regReq, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Register expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var regResp struct {
		Message string `json:"message"`
		Data    struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &regResp); err != nil {
		t.Fatalf("Failed to unmarshal register response: %v", err)
	}
	if regResp.Data.ID == "" || regResp.Data.Nickname != "gacha_fan" {
		t.Fatalf("Unexpected register data: %+v", regResp.Data)
	}
	userID := regResp.Data.ID
	loginReq := map[string]string{
		"id":       userID,
		"password": "secretpassword",
	}
	w = performRequest(router, "POST", "/api/login", loginReq, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Login expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var loginResp struct {
		Message string `json:"message"`
		Token   string `json:"token"`
		User    struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
			Role     string `json:"role"`
			Bio      string `json:"bio"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("Failed to unmarshal login response: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatal("Expected non-empty JWT token on login")
	}
	if loginResp.User.Bio != "Testing gacha simulator" {
		t.Fatalf("Expected bio 'Testing gacha simulator', got '%s'", loginResp.User.Bio)
	}
	userToken := loginResp.Token

	// Test login with nickname fallback
	loginNickReq := map[string]string{
		"id":       "gacha_fan",
		"password": "secretpassword",
	}
	wNick := performRequest(router, "POST", "/api/login", loginNickReq, nil)
	if wNick.Code != http.StatusOK {
		t.Fatalf("Login with nickname expected 200, got %d: %s", wNick.Code, wNick.Body.String())
	}
	var loginNickResp struct {
		Token string `json:"token"`
		User  struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		} `json:"user"`
	}
	if err := json.Unmarshal(wNick.Body.Bytes(), &loginNickResp); err != nil {
		t.Fatalf("Failed to unmarshal login with nickname response: %v", err)
	}
	if loginNickResp.Token == "" || loginNickResp.User.ID != userID || loginNickResp.User.Nickname != "gacha_fan" {
		t.Fatalf("Unexpected login with nickname data: %+v", loginNickResp)
	}

	authHeader := map[string]string{"Authorization": "Bearer " + userToken}
	w = performRequest(router, "GET", "/api/user/me", nil, authHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/user/me expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var meResp struct {
		UserID   string `json:"user_id"`
		Nickname string `json:"nickname"`
		Role     string `json:"role"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("Failed to unmarshal me response: %v", err)
	}
	if meResp.UserID != userID || meResp.Role != "user" || meResp.Nickname != "gacha_fan" {
		t.Fatalf("Unexpected me response: %+v", meResp)
	}
	updateReq := map[string]string{
		"nickname": "lucky_puller",
		"bio":      "Always pulling S tier",
	}
	w = performRequest(router, "PUT", "/api/user/profile", updateReq, authHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT /api/user/profile expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updateResp struct {
		Message string `json:"message"`
		Data    User   `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("Failed to unmarshal update profile response: %v", err)
	}
	if updateResp.Data.Nickname != "lucky_puller" || updateResp.Data.Bio != "Always pulling S tier" {
		t.Fatalf("Profile update not reflected: %+v", updateResp.Data)
	}
}

func TestAdminFlow(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()
	loginReq := map[string]string{
		"id":       "admin",
		"password": "admin123",
	}
	w := performRequest(router, "POST", "/api/login", loginReq, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Admin login expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var loginResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	adminToken := loginResp.Token
	adminHeader := map[string]string{"Authorization": "Bearer " + adminToken}
	createChar := func(name, rarity string, limited bool) uint {
		req := map[string]interface{}{
			"name":       name,
			"rarity":     rarity,
			"is_limited": limited,
		}
		w := performRequest(router, "POST", "/api/admin/character", req, adminHeader)
		if w.Code != http.StatusOK {
			t.Fatalf("Create character %s expected 200, got %d: %s", name, w.Code, w.Body.String())
		}
		var charResp struct {
			Data Character `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &charResp)
		return charResp.Data.ID
	}

	charS1ID := createChar("Seele", "S", true)
	charS2ID := createChar("Bronya", "S", false)
	charA1ID := createChar("Natasha", "A", false)
	charB1ID := createChar("TrashCan", "B", false)
	pushToPool := func(charID uint, isUp bool) {
		req := map[string]interface{}{
			"character_id": charID,
			"is_up":        isUp,
		}
		w := performRequest(router, "POST", "/api/admin/pool/push", req, adminHeader)
		if w.Code != http.StatusOK {
			t.Fatalf("Push character %d expected 200, got %d: %s", charID, w.Code, w.Body.String())
		}
	}

	pushToPool(charS1ID, true)
	pushToPool(charS2ID, false)
	pushToPool(charA1ID, false)
	pushToPool(charB1ID, false)
	w = performRequest(router, "GET", "/api/pool/info", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/pool/info expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var poolInfoResp struct {
		Config PoolConfig `json:"config"`
		Banner struct {
			UpCharacter       *Character  `json:"up_character"`
			LimitedSCharacter []Character `json:"limited_s_character"`
			StandardPoolCount int         `json:"standard_pool_count"`
		} `json:"banner"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &poolInfoResp); err != nil {
		t.Fatalf("Failed to unmarshal pool info response: %v", err)
	}
	if poolInfoResp.Banner.UpCharacter == nil || poolInfoResp.Banner.UpCharacter.Name != "Seele" {
		t.Fatalf("Expected UP character 'Seele', got: %+v", poolInfoResp.Banner.UpCharacter)
	}
	if len(poolInfoResp.Banner.LimitedSCharacter) != 1 {
		t.Fatalf("Expected 1 limited S character, got %d", len(poolInfoResp.Banner.LimitedSCharacter))
	}
	if poolInfoResp.Banner.StandardPoolCount != 3 { // Bronya, Natasha, TrashCan
		t.Fatalf("Expected 3 standard characters in pool, got %d", poolInfoResp.Banner.StandardPoolCount)
	}
}

func TestGachaFlow(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()
	loginAdminReq := map[string]string{"id": "admin", "password": "admin123"}
	w := performRequest(router, "POST", "/api/login", loginAdminReq, nil)
	var adminLoginResp struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &adminLoginResp)
	adminHeader := map[string]string{"Authorization": "Bearer " + adminLoginResp.Token}

	createAndPush := func(name, rarity string, limited, isUp bool) uint {
		charReq := map[string]interface{}{"name": name, "rarity": rarity, "is_limited": limited}
		w := performRequest(router, "POST", "/api/admin/character", charReq, adminHeader)
		var cResp struct{ Data Character }
		_ = json.Unmarshal(w.Body.Bytes(), &cResp)
		pushReq := map[string]interface{}{"character_id": cResp.Data.ID, "is_up": isUp}
		performRequest(router, "POST", "/api/admin/pool/push", pushReq, adminHeader)
		return cResp.Data.ID
	}

	createAndPush("Acheron", "S", true, true)
	createAndPush("Himeko", "S", false, false)
	createAndPush("March 7th", "A", false, false)
	createAndPush("Light Cone B", "B", false, false)
	userReg := map[string]string{"nickname": "gacha_tester", "password": "pw", "bio": "tester"}
	w = performRequest(router, "POST", "/api/register", userReg, nil)
	var regResp struct {
		Data struct{ ID string }
	}
	_ = json.Unmarshal(w.Body.Bytes(), &regResp)
	userID := regResp.Data.ID

	userLogin := map[string]string{"id": userID, "password": "pw"}
	w = performRequest(router, "POST", "/api/login", userLogin, nil)
	var userLoginResp struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &userLoginResp)
	userHeader := map[string]string{"Authorization": "Bearer " + userLoginResp.Token}
	draw1Req := map[string]int{"count": 1}
	w = performRequest(router, "POST", "/api/gacha/draw", draw1Req, userHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("1-pull expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var draw1Resp struct {
		Count   int          `json:"count"`
		Results []DrawResult `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &draw1Resp); err != nil {
		t.Fatalf("Failed to parse draw 1 response: %v", err)
	}
	if draw1Resp.Count != 1 || len(draw1Resp.Results) != 1 {
		t.Fatalf("Expected 1 result in 1-pull, got %d", len(draw1Resp.Results))
	}
	if draw1Resp.Results[0].Character.Name == "" {
		t.Fatal("Expected valid character in draw result")
	}
	draw10Req := map[string]int{"count": 10}
	w = performRequest(router, "POST", "/api/gacha/draw", draw10Req, userHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("10-pull expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var draw10Resp struct {
		Count   int          `json:"count"`
		Results []DrawResult `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &draw10Resp); err != nil {
		t.Fatalf("Failed to parse draw 10 response: %v", err)
	}
	if draw10Resp.Count != 10 || len(draw10Resp.Results) != 10 {
		t.Fatalf("Expected 10 results in 10-pull, got %d", len(draw10Resp.Results))
	}
	var dbUser User
	if err := DB.Where("id = ?", userID).First(&dbUser).Error; err != nil {
		t.Fatalf("Failed to query user: %v", err)
	}
	if dbUser.PitySCount < 0 || dbUser.PityACount < 0 {
		t.Fatalf("Invalid pity counters: S=%d, A=%d", dbUser.PitySCount, dbUser.PityACount)
	}
	w = performRequest(router, "GET", "/api/gacha/inventory", nil, userHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/gacha/inventory expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var invResp struct {
		Total int             `json:"total"`
		Data  []UserCharacter `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &invResp); err != nil {
		t.Fatalf("Failed to parse inventory: %v", err)
	}
	if invResp.Total == 0 || len(invResp.Data) == 0 {
		t.Fatal("Expected at least one character in inventory after 11 pulls")
	}
	for _, uc := range invResp.Data {
		if uc.Character.Name == "" {
			t.Fatalf("UserCharacter relationship not preloaded properly: %+v", uc)
		}
	}
	w = performRequest(router, "GET", "/api/gacha/history", nil, userHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/gacha/history expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var histResp struct {
		Total    int64         `json:"total"`
		Page     int           `json:"page"`
		PageSize int           `json:"page_size"`
		Data     []GachaRecord `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &histResp); err != nil {
		t.Fatalf("Failed to parse history: %v", err)
	}
	if histResp.Total != 11 || len(histResp.Data) != 11 {
		t.Fatalf("Expected 11 history records, got total=%d, data_len=%d", histResp.Total, len(histResp.Data))
	}
	w = performRequest(router, "GET", "/api/gacha/stats", nil, userHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/gacha/stats expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var statsResp struct {
		TotalSCount int         `json:"total_s_count"`
		History     []SStatItem `json:"history"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &statsResp); err != nil {
		t.Fatalf("Failed to parse stats: %v", err)
	}
	if statsResp.TotalSCount != len(statsResp.History) {
		t.Fatalf("Stats count mismatch: total=%d, len=%d", statsResp.TotalSCount, len(statsResp.History))
	}
	w = performRequest(router, "DELETE", "/api/gacha/history", nil, userHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("DELETE /api/gacha/history expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var recordCount int64
	DB.Model(&GachaRecord{}).Where("user_id = ?", userID).Count(&recordCount)
	if recordCount != 0 {
		t.Fatalf("Expected 0 gacha records after clear, got %d", recordCount)
	}

	var userCharCount int64
	DB.Model(&UserCharacter{}).Where("user_id = ?", userID).Count(&userCharCount)
	if userCharCount != 0 {
		t.Fatalf("Expected 0 user characters after clear, got %d", userCharCount)
	}

	if err := DB.Where("id = ?", userID).First(&dbUser).Error; err != nil {
		t.Fatalf("Failed to query user: %v", err)
	}
	if dbUser.PitySCount != 0 || dbUser.PityACount != 0 {
		t.Fatalf("Expected pity counts to reset to 0, got S=%d, A=%d", dbUser.PitySCount, dbUser.PityACount)
	}
	w = performRequest(router, "GET", "/api/gacha/inventory", nil, userHeader)
	_ = json.Unmarshal(w.Body.Bytes(), &invResp)
	if invResp.Total != 0 {
		t.Fatalf("Expected 0 inventory items after clear, got %d", invResp.Total)
	}
}

func TestHardPityGuarantee(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	loginAdminReq := map[string]string{"id": "admin", "password": "admin123"}
	w := performRequest(router, "POST", "/api/login", loginAdminReq, nil)
	var adminLoginResp struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &adminLoginResp)
	adminHeader := map[string]string{"Authorization": "Bearer " + adminLoginResp.Token}
	for _, r := range []string{"S", "A", "B"} {
		w := performRequest(router, "POST", "/api/admin/character", map[string]interface{}{
			"name": fmt.Sprintf("Char_%s", r), "rarity": r, "is_limited": false,
		}, adminHeader)
		var cResp struct{ Data Character }
		_ = json.Unmarshal(w.Body.Bytes(), &cResp)
		performRequest(router, "POST", "/api/admin/pool/push", map[string]interface{}{
			"character_id": cResp.Data.ID, "is_up": false,
		}, adminHeader)
	}
	w = performRequest(router, "POST", "/api/register", map[string]string{
		"nickname": "pity_hero", "password": "pw",
	}, nil)
	var regResp struct{ Data struct{ ID string } }
	_ = json.Unmarshal(w.Body.Bytes(), &regResp)
	userID := regResp.Data.ID
	DB.Model(&User{}).Where("id = ?", userID).Update("pity_s_count", 79)

	w = performRequest(router, "POST", "/api/login", map[string]string{
		"id": userID, "password": "pw",
	}, nil)
	var loginResp struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	userHeader := map[string]string{"Authorization": "Bearer " + loginResp.Token}
	w = performRequest(router, "POST", "/api/gacha/draw", map[string]int{"count": 1}, userHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("Draw failed: %s", w.Body.String())
	}
	var drawResp struct {
		Results []DrawResult `json:"results"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &drawResp)
	if len(drawResp.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(drawResp.Results))
	}
	if drawResp.Results[0].Character.Rarity != "S" {
		t.Fatalf("Expected hard pity to guarantee rarity S, got: %s", drawResp.Results[0].Character.Rarity)
	}

	var u User
	DB.Where("id = ?", userID).First(&u)
	if u.PitySCount != 0 {
		t.Fatalf("Expected pity_s_count to reset to 0 after pulling S, got %d", u.PitySCount)
	}
}

func TestAuthAndPermissionEdgeCases(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()
	w := performRequest(router, "GET", "/api/user/me", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized without token, got %d", w.Code)
	}
	w = performRequest(router, "GET", "/api/user/me", nil, map[string]string{"Authorization": "Bearer invalid.token.value"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized with invalid token, got %d", w.Code)
	}
	userReg := map[string]string{"nickname": "regular_user", "password": "pw", "bio": "bio"}
	w = performRequest(router, "POST", "/api/register", userReg, nil)
	var regResp struct {
		Data struct{ ID string }
	}
	_ = json.Unmarshal(w.Body.Bytes(), &regResp)

	userLogin := map[string]string{"id": regResp.Data.ID, "password": "pw"}
	w = performRequest(router, "POST", "/api/login", userLogin, nil)
	var loginResp struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	userHeader := map[string]string{"Authorization": "Bearer " + loginResp.Token}

	w = performRequest(router, "POST", "/api/admin/character", map[string]interface{}{
		"name": "Hacker", "rarity": "S",
	}, userHeader)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden when normal user accesses admin endpoint, got %d", w.Code)
	}

	// 4. Invalid draw count (e.g., 5 pulls) -> 400 Bad Request
	w = performRequest(router, "POST", "/api/gacha/draw", map[string]int{"count": 5}, userHeader)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for draw count 5, got %d", w.Code)
	}
}

func TestPaginationEdgeCases(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	userReg := map[string]string{"nickname": "pager_user", "password": "pw", "bio": "pager"}
	w := performRequest(router, "POST", "/api/register", userReg, nil)
	var regResp struct {
		Data struct{ ID string }
	}
	_ = json.Unmarshal(w.Body.Bytes(), &regResp)
	userID := regResp.Data.ID

	userLogin := map[string]string{"id": userID, "password": "pw"}
	w = performRequest(router, "POST", "/api/login", userLogin, nil)
	var loginResp struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	userHeader := map[string]string{"Authorization": "Bearer " + loginResp.Token}

	// Test page=0&page_size=-5 (should clamp to page=1, page_size=1)
	w = performRequest(router, "GET", "/api/gacha/history?page=0&page_size=-5", nil, userHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for page=0&page_size=-5, got %d: %s", w.Code, w.Body.String())
	}
	var histResp struct {
		Page     int           `json:"page"`
		PageSize int           `json:"page_size"`
		Data     []GachaRecord `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &histResp); err != nil {
		t.Fatalf("Failed to parse history: %v", err)
	}
	if histResp.Page != 1 || histResp.PageSize != 1 {
		t.Fatalf("Expected clamped page=1, page_size=1, got page=%d, page_size=%d", histResp.Page, histResp.PageSize)
	}

	// Test page_size > 100 clamping (e.g. page_size=200 -> 100)
	w = performRequest(router, "GET", "/api/gacha/history?page=1&page_size=200", nil, userHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for page_size=200, got %d: %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &histResp)
	if histResp.PageSize != 100 {
		t.Fatalf("Expected clamped page_size=100, got %d", histResp.PageSize)
	}
}

func TestConcurrentDraw(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	loginAdminReq := map[string]string{"id": "admin", "password": "admin123"}
	w := performRequest(router, "POST", "/api/login", loginAdminReq, nil)
	var adminLoginResp struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &adminLoginResp)
	adminHeader := map[string]string{"Authorization": "Bearer " + adminLoginResp.Token}

	for _, r := range []string{"S", "A", "B"} {
		w := performRequest(router, "POST", "/api/admin/character", map[string]interface{}{
			"name": fmt.Sprintf("Char_%s", r), "rarity": r, "is_limited": false,
		}, adminHeader)
		var cResp struct{ Data Character }
		_ = json.Unmarshal(w.Body.Bytes(), &cResp)
		performRequest(router, "POST", "/api/admin/pool/push", map[string]interface{}{
			"character_id": cResp.Data.ID, "is_up": false,
		}, adminHeader)
	}

	userReg := map[string]string{"nickname": "concurrent_user", "password": "pw", "bio": "concurrent"}
	w = performRequest(router, "POST", "/api/register", userReg, nil)
	var regResp struct {
		Data struct{ ID string }
	}
	_ = json.Unmarshal(w.Body.Bytes(), &regResp)
	userID := regResp.Data.ID

	userLogin := map[string]string{"id": userID, "password": "pw"}
	w = performRequest(router, "POST", "/api/login", userLogin, nil)
	var loginResp struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	userHeader := map[string]string{"Authorization": "Bearer " + loginResp.Token}

	var wg sync.WaitGroup
	errChan := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := performRequest(router, "POST", "/api/gacha/draw", map[string]int{"count": 10}, userHeader)
			if rec.Code != http.StatusOK {
				errChan <- fmt.Errorf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
			}
		}()
	}
	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Fatalf("Concurrent draw failed: %v", err)
	}

	var totalRecords int64
	DB.Model(&GachaRecord{}).Where("user_id = ?", userID).Count(&totalRecords)
	if totalRecords != 50 {
		t.Fatalf("Expected exactly 50 gacha records, got %d", totalRecords)
	}
}

type MockEmailSender struct {
	mu        sync.Mutex
	SentCodes map[string]string
}

func (m *MockEmailSender) SendCode(email, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SentCodes == nil {
		m.SentCodes = make(map[string]string)
	}
	m.SentCodes[email] = code
	return nil
}

func (m *MockEmailSender) GetCode(email string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.SentCodes[email]
}

type MockOAuthProvider struct {
	AuthURL     string
	Token       string
	Profile     *GithubProfile
	ExchangeErr error
	ProfileErr  error
}

func (m *MockOAuthProvider) GetAuthURL(state string) string {
	if m.AuthURL != "" {
		return m.AuthURL
	}
	return "https://github.com/login/oauth/authorize?mock=true&state=" + state
}

func (m *MockOAuthProvider) ExchangeCode(code string) (string, error) {
	if m.ExchangeErr != nil {
		return "", m.ExchangeErr
	}
	if code == "invalid_code" {
		return "", errors.New("bad verification code")
	}
	return m.Token, nil
}

func (m *MockOAuthProvider) GetUserProfile(token string) (*GithubProfile, error) {
	if m.ProfileErr != nil {
		return nil, m.ProfileErr
	}
	return m.Profile, nil
}

func TestEmailLogin_Flow(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	mockEmail := &MockEmailSender{SentCodes: make(map[string]string)}
	origSender := CurrentEmailSender
	CurrentEmailSender = mockEmail
	defer func() {
		CurrentEmailSender = origSender
		ClearCodeStore()
	}()

	testEmail := "traveler@genshin.com"

	// 1. Send code -> mock records it
	sendReq := map[string]string{"email": testEmail}
	w := performRequest(router, "POST", "/api/auth/email/send-code", sendReq, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for send-code, got %d: %s", w.Code, w.Body.String())
	}
	code := mockEmail.GetCode(testEmail)
	if len(code) != 6 {
		t.Fatalf("Expected 6-digit code, got '%s'", code)
	}

	// 2. Login with wrong code -> returns 400
	wrongCode := "000000"
	if code == "000000" {
		wrongCode = "111111"
	}
	wrongLoginReq := map[string]string{"email": testEmail, "code": wrongCode}
	w = performRequest(router, "POST", "/api/auth/email/login", wrongLoginReq, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for wrong code, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Login with correct code (new user) -> returns 200 + JWT, creates user with email
	correctLoginReq := map[string]string{"email": testEmail, "code": code}
	w = performRequest(router, "POST", "/api/auth/email/login", correctLoginReq, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for correct code, got %d: %s", w.Code, w.Body.String())
	}

	var loginResp struct {
		Message string `json:"message"`
		Token   string `json:"token"`
		User    struct {
			ID       string  `json:"id"`
			Nickname string  `json:"nickname"`
			Role     string  `json:"role"`
			Email    *string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatalf("Expected non-empty token")
	}
	if loginResp.User.Email == nil || *loginResp.User.Email != testEmail {
		t.Fatalf("Expected user email to be %s, got %v", testEmail, loginResp.User.Email)
	}
	userID := loginResp.User.ID

	// Verify user in DB
	var dbUser User
	if err := DB.Where("id = ?", userID).First(&dbUser).Error; err != nil {
		t.Fatalf("User not found in DB: %v", err)
	}
	if dbUser.Email == nil || *dbUser.Email != testEmail {
		t.Fatalf("DB user email mismatch: %v", dbUser.Email)
	}

	// 4. Use JWT to call /api/user/me -> succeeds
	authHeader := map[string]string{"Authorization": "Bearer " + loginResp.Token}
	w = performRequest(router, "GET", "/api/user/me", nil, authHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /api/user/me, got %d: %s", w.Code, w.Body.String())
	}
	var meResp struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("Failed to parse me response: %v", err)
	}
	if meResp.UserID != userID {
		t.Fatalf("Expected user_id %s, got %s", userID, meResp.UserID)
	}

	// 5. Code cannot be reused -> returns 400
	w = performRequest(router, "POST", "/api/auth/email/login", correctLoginReq, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for reused code, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Send new code -> login existing user -> returns 200 + JWT for same user ID
	w = performRequest(router, "POST", "/api/auth/email/send-code", sendReq, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for second send-code, got %d: %s", w.Code, w.Body.String())
	}
	secondCode := mockEmail.GetCode(testEmail)
	secondLoginReq := map[string]string{"email": testEmail, "code": secondCode}
	w = performRequest(router, "POST", "/api/auth/email/login", secondLoginReq, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for second login, got %d: %s", w.Code, w.Body.String())
	}
	var secondLoginResp struct {
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &secondLoginResp); err != nil {
		t.Fatalf("Failed to parse second login response: %v", err)
	}
	if secondLoginResp.User.ID != userID {
		t.Fatalf("Expected same user ID %s, got %s", userID, secondLoginResp.User.ID)
	}
}

func TestGithubOAuth_Flow(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	mockOAuth := &MockOAuthProvider{
		Token: "mock_gh_token",
		Profile: &GithubProfile{
			ID:        123456,
			Login:     "octocat",
			Name:      "The Octocat",
			Email:     "octocat@github.com",
			AvatarURL: "https://github.com/images/error/octocat_happy.gif",
		},
	}
	origOAuth := CurrentOAuthProvider
	CurrentOAuthProvider = mockOAuth
	defer func() {
		CurrentOAuthProvider = origOAuth
	}()

	// 1. /api/auth/github/login returns URL
	w := performRequest(router, "GET", "/api/auth/github/login", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for github login URL, got %d: %s", w.Code, w.Body.String())
	}
	var loginURLResp struct {
		AuthURL string `json:"auth_url"`
		State   string `json:"state"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loginURLResp); err != nil {
		t.Fatalf("Failed to parse github login url response: %v", err)
	}
	if loginURLResp.AuthURL == "" || loginURLResp.State == "" {
		t.Fatalf("Expected auth_url and state, got %+v", loginURLResp)
	}

	// 2. Callback with valid code -> creates user with github_id, returns 200 + JWT
	callbackReq := map[string]string{
		"code":  "valid_code_123",
		"state": loginURLResp.State,
	}
	w = performRequest(router, "POST", "/api/auth/github/callback", callbackReq, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for github callback, got %d: %s", w.Code, w.Body.String())
	}

	var cbResp struct {
		Message string `json:"message"`
		Token   string `json:"token"`
		User    struct {
			ID       string  `json:"id"`
			Nickname string  `json:"nickname"`
			Role     string  `json:"role"`
			GithubID *string `json:"github_id"`
			Email    *string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cbResp); err != nil {
		t.Fatalf("Failed to parse callback response: %v", err)
	}
	if cbResp.Token == "" {
		t.Fatalf("Expected non-empty token")
	}
	if cbResp.User.GithubID == nil || *cbResp.User.GithubID != "123456" {
		t.Fatalf("Expected github_id 123456, got %v", cbResp.User.GithubID)
	}
	firstUserID := cbResp.User.ID

	// Verify in DB
	var dbUser User
	if err := DB.Where("id = ?", firstUserID).First(&dbUser).Error; err != nil {
		t.Fatalf("User not found in DB: %v", err)
	}
	if dbUser.GithubID == nil || *dbUser.GithubID != "123456" {
		t.Fatalf("Expected DB github_id 123456, got %v", dbUser.GithubID)
	}

	// 3. Subsequent callback with same GitHub ID -> logs in existing user
	w = performRequest(router, "GET", "/api/auth/github/callback?code=valid_code_123", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for subsequent github callback, got %d: %s", w.Code, w.Body.String())
	}
	var secondCbResp struct {
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &secondCbResp); err != nil {
		t.Fatalf("Failed to parse second callback response: %v", err)
	}
	if secondCbResp.User.ID != firstUserID {
		t.Fatalf("Expected same user ID %s on repeated login, got %s", firstUserID, secondCbResp.User.ID)
	}

	// 3b. Browser callback with Accept: text/html -> returns HTML with localStorage and redirect
	w = performRequest(router, "GET", "/api/auth/github/callback?code=valid_code_123", nil, map[string]string{
		"Accept": "text/html,application/xhtml+xml",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for browser HTML github callback, got %d: %s", w.Code, w.Body.String())
	}
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("Expected Content-Type to contain text/html, got %s", contentType)
	}
	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "localStorage.setItem('gacha_token'") {
		t.Fatalf("Expected HTML body to contain localStorage.setItem('gacha_token'), got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "localStorage.setItem('gacha_user', JSON.stringify(") {
		t.Fatalf("Expected HTML body to contain localStorage.setItem('gacha_user', JSON.stringify(, got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "window.location.href = '/'") {
		t.Fatalf("Expected HTML body to contain window.location.href = '/', got %s", bodyStr)
	}

	// 4. Callback with GitHub profile containing existing email -> links github_id to existing account
	linkedEmail := "linked@domain.com"
	existingUser := User{
		ID:       "existing1",
		Nickname: "PreExistingUser",
		Password: "password_hash",
		Email:    &linkedEmail,
		Role:     "user",
	}
	if err := DB.Create(&existingUser).Error; err != nil {
		t.Fatalf("Failed to create pre-existing user: %v", err)
	}

	mockOAuth.Profile = &GithubProfile{
		ID:    999999,
		Login: "newghlogin",
		Email: linkedEmail,
	}

	w = performRequest(router, "POST", "/api/auth/github/callback", map[string]string{"code": "link_code"}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for linking callback, got %d: %s", w.Code, w.Body.String())
	}
	var linkCbResp struct {
		User struct {
			ID       string  `json:"id"`
			GithubID *string `json:"github_id"`
			Email    *string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &linkCbResp); err != nil {
		t.Fatalf("Failed to parse link callback response: %v", err)
	}
	if linkCbResp.User.ID != "existing1" {
		t.Fatalf("Expected linked user ID 'existing1', got %s", linkCbResp.User.ID)
	}
	if linkCbResp.User.GithubID == nil || *linkCbResp.User.GithubID != "999999" {
		t.Fatalf("Expected github_id 999999, got %v", linkCbResp.User.GithubID)
	}

	var verifyUser User
	if err := DB.Where("id = ?", "existing1").First(&verifyUser).Error; err != nil {
		t.Fatalf("Failed to find linked user in DB: %v", err)
	}
	if verifyUser.GithubID == nil || *verifyUser.GithubID != "999999" {
		t.Fatalf("Expected DB github_id 999999, got %v", verifyUser.GithubID)
	}

	// 5. Callback with invalid code -> returns 400/401 error
	w = performRequest(router, "POST", "/api/auth/github/callback", map[string]string{"code": "invalid_code"}, nil)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 400 or 401 for invalid code, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGithubOAuth_SingleQuoteNickname(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	mockOAuth := &MockOAuthProvider{
		Token: "mock_gh_token_quote",
		Profile: &GithubProfile{
			ID:        789101,
			Login:     "O'Connor",
			Name:      "Arthur O'Connor",
			Email:     "oconnor@example.com",
			AvatarURL: "https://github.com/images/error/oconnor.gif",
		},
	}
	origOAuth := CurrentOAuthProvider
	CurrentOAuthProvider = mockOAuth
	defer func() {
		CurrentOAuthProvider = origOAuth
	}()

	w := performRequest(router, "GET", "/api/auth/github/callback?code=quote_code", nil, map[string]string{
		"Accept": "text/html,application/xhtml+xml",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for github callback with single-quote nickname, got %d: %s", w.Code, w.Body.String())
	}
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("Expected Content-Type to contain text/html, got %s", contentType)
	}
	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "JSON.stringify(") {
		t.Fatalf("Expected HTML body to contain JSON.stringify(, got %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "O'Connor") {
		t.Fatalf("Expected HTML body to contain O'Connor, got %s", bodyStr)
	}
}

func TestGithubOAuth_DevMockMode(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	origClientID := os.Getenv("GITHUB_CLIENT_ID")
	origClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	origDevMock := os.Getenv("GITHUB_DEV_MOCK")
	os.Unsetenv("GITHUB_CLIENT_ID")
	os.Unsetenv("GITHUB_CLIENT_SECRET")
	os.Setenv("GITHUB_DEV_MOCK", "true")
	defer func() {
		if origClientID != "" {
			os.Setenv("GITHUB_CLIENT_ID", origClientID)
		} else {
			os.Unsetenv("GITHUB_CLIENT_ID")
		}
		if origClientSecret != "" {
			os.Setenv("GITHUB_CLIENT_SECRET", origClientSecret)
		} else {
			os.Unsetenv("GITHUB_CLIENT_SECRET")
		}
		if origDevMock != "" {
			os.Setenv("GITHUB_DEV_MOCK", origDevMock)
		} else {
			os.Unsetenv("GITHUB_DEV_MOCK")
		}
	}()

	client := NewGithubOAuthClient()
	if !client.IsMock() {
		t.Fatalf("Expected client.IsMock() to be true in dev mock mode")
	}

	origOAuth := CurrentOAuthProvider
	CurrentOAuthProvider = client
	defer func() {
		CurrentOAuthProvider = origOAuth
	}()

	// /api/auth/github/login returns mock redirect URL containing code=mock_code_dev
	w := performRequest(router, "GET", "/api/auth/github/login", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for github login, got %d: %s", w.Code, w.Body.String())
	}
	var loginResp struct {
		AuthURL string `json:"auth_url"`
		State   string `json:"state"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}
	if !strings.Contains(loginResp.AuthURL, "code=mock_code_dev") {
		t.Fatalf("Expected AuthURL to contain code=mock_code_dev, got %s", loginResp.AuthURL)
	}

	// Calling /api/auth/github/callback?code=mock_code_dev succeeds with status 200 and returns JWT and mock user
	w = performRequest(router, "GET", "/api/auth/github/callback?code=mock_code_dev", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for callback, got %d: %s", w.Code, w.Body.String())
	}
	var cbResp struct {
		Message string `json:"message"`
		Token   string `json:"token"`
		User    struct {
			ID       string  `json:"id"`
			Nickname string  `json:"nickname"`
			Role     string  `json:"role"`
			GithubID *string `json:"github_id"`
			Email    *string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cbResp); err != nil {
		t.Fatalf("Failed to parse callback response: %v", err)
	}
	if cbResp.Token == "" {
		t.Fatalf("Expected non-empty JWT token")
	}
	if cbResp.User.GithubID == nil || *cbResp.User.GithubID != "999999" {
		t.Fatalf("Expected github_id '999999', got %v", cbResp.User.GithubID)
	}
	if cbResp.User.Nickname != "github_mock_user" {
		t.Fatalf("Expected nickname 'github_mock_user', got %s", cbResp.User.Nickname)
	}
}

type trackingRoundTripper struct {
	called  bool
	lastReq *http.Request
}

func (t *trackingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	t.called = true
	t.lastReq = req
	return nil, errors.New("simulated network error")
}

func TestGithubOAuth_RealModeAttemptsRequest(t *testing.T) {
	origClientID := os.Getenv("GITHUB_CLIENT_ID")
	origClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	origDevMock := os.Getenv("GITHUB_DEV_MOCK")

	os.Setenv("GITHUB_CLIENT_ID", "real_dummy_id")
	os.Setenv("GITHUB_CLIENT_SECRET", "real_dummy_secret")
	os.Unsetenv("GITHUB_DEV_MOCK")
	defer func() {
		if origClientID != "" {
			os.Setenv("GITHUB_CLIENT_ID", origClientID)
		} else {
			os.Unsetenv("GITHUB_CLIENT_ID")
		}
		if origClientSecret != "" {
			os.Setenv("GITHUB_CLIENT_SECRET", origClientSecret)
		} else {
			os.Unsetenv("GITHUB_CLIENT_SECRET")
		}
		if origDevMock != "" {
			os.Setenv("GITHUB_DEV_MOCK", origDevMock)
		} else {
			os.Unsetenv("GITHUB_DEV_MOCK")
		}
	}()

	client := NewGithubOAuthClient()
	if client.IsMock() {
		t.Fatalf("Expected client.IsMock() to be false when real credentials are configured")
	}

	tracker := &trackingRoundTripper{}
	client.HTTPClient = &http.Client{Transport: tracker}

	authURL := client.GetAuthURL("state123")
	if !strings.Contains(authURL, "https://github.com/login/oauth/authorize") {
		t.Fatalf("Expected authURL to contain https://github.com/login/oauth/authorize, got %s", authURL)
	}

	tracker.called = false
	_, err := client.ExchangeCode("invalid_code")
	if err == nil {
		t.Fatalf("Expected error when attempting external request in real mode with invalid code")
	}
	if !tracker.called {
		t.Fatalf("Expected RoundTrip to be called for invalid_code")
	}

	// Verify that code="mock_bypass" does NOT bypass real HTTP request in real mode
	tracker.called = false
	_, err = client.ExchangeCode("mock_bypass")
	if err == nil {
		t.Fatalf("Expected error when attempting external request in real mode with mock_bypass code")
	}
	if !tracker.called {
		t.Fatalf("Expected RoundTrip to be called for mock_bypass code")
	}

	// Verify that token="mock_bypass" does NOT bypass real HTTP request in real mode
	tracker.called = false
	_, err = client.GetUserProfile("mock_bypass")
	if err == nil {
		t.Fatalf("Expected error when attempting external request in real mode with mock_bypass token")
	}
	if !tracker.called {
		t.Fatalf("Expected RoundTrip to be called for mock_bypass token")
	}
}

func TestLoadPresetsFlow(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	// 1. Login as default admin
	loginAdminReq := map[string]string{"id": "admin", "password": "admin123"}
	w := performRequest(router, "POST", "/api/login", loginAdminReq, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Admin login expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var adminLoginResp struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &adminLoginResp)
	adminHeader := map[string]string{"Authorization": "Bearer " + adminLoginResp.Token}

	// 2. Register and login a normal user
	regReq := map[string]string{
		"nickname": "preset_tester",
		"password": "testerpassword",
	}
	w = performRequest(router, "POST", "/api/register", regReq, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("User register expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var userRegResp struct {
		Data struct{ ID string } `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &userRegResp)

	loginUserReq := map[string]string{"id": userRegResp.Data.ID, "password": "testerpassword"}
	w = performRequest(router, "POST", "/api/login", loginUserReq, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("User login expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var userLoginResp struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &userLoginResp)
	userHeader := map[string]string{"Authorization": "Bearer " + userLoginResp.Token}

	// Permission check: normal user cannot call load-presets
	w = performRequest(router, "POST", "/api/admin/pool/load-presets", nil, userHeader)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Normal user calling load-presets expected 403 Forbidden, got %d", w.Code)
	}

	// Invalid file path returns 400
	w = performRequest(router, "POST", "/api/admin/pool/load-presets", map[string]string{"file_path": "nonexistent_file.json"}, adminHeader)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Nonexistent file path expected 400 Bad Request, got %d", w.Code)
	}

	// Path traversal attempt returns 400
	w = performRequest(router, "POST", "/api/admin/pool/load-presets", map[string]string{"file_path": "../../etc/passwd"}, adminHeader)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Path traversal expected 400 Bad Request, got %d", w.Code)
	}

	// 3. Admin loads presets (default file)
	w = performRequest(router, "POST", "/api/admin/pool/load-presets", nil, adminHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("Admin load-presets expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var loadResp struct {
		Message string      `json:"message"`
		Count   int         `json:"count"`
		Data    []Character `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loadResp); err != nil {
		t.Fatalf("Failed to parse load presets response: %v", err)
	}
	if loadResp.Count != 10 {
		t.Fatalf("Expected 10 characters loaded, got %d", loadResp.Count)
	}

	// 4. Verify pool info via GET /api/pool/info
	w = performRequest(router, "GET", "/api/pool/info", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/pool/info expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var poolInfoResp struct {
		Config PoolConfig `json:"config"`
		Banner struct {
			UpCharacter       *Character  `json:"up_character"`
			LimitedSCharacter []Character `json:"limited_s_character"`
			StandardPoolCount int         `json:"standard_pool_count"`
		} `json:"banner"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &poolInfoResp); err != nil {
		t.Fatalf("Failed to unmarshal pool info response: %v", err)
	}
	if poolInfoResp.Banner.UpCharacter == nil {
		t.Fatalf("Expected UP character, got nil")
	}
	if poolInfoResp.Banner.UpCharacter.Name != "星渊猎手·卡莲" {
		t.Fatalf("Expected UP character '星渊猎手·卡莲', got '%s'", poolInfoResp.Banner.UpCharacter.Name)
	}
	if len(poolInfoResp.Banner.LimitedSCharacter) != 2 {
		t.Fatalf("Expected 2 limited S characters, got %d", len(poolInfoResp.Banner.LimitedSCharacter))
	}

	// 5. Normal user draws 10-pull
	drawReq := map[string]int{"count": 10}
	w = performRequest(router, "POST", "/api/gacha/draw", drawReq, userHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("10-pull draw expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var drawResp struct {
		Message string       `json:"message"`
		Count   int          `json:"count"`
		Results []DrawResult `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &drawResp); err != nil {
		t.Fatalf("Failed to unmarshal draw response: %v", err)
	}
	if len(drawResp.Results) != 10 {
		t.Fatalf("Expected 10 draw results, got %d", len(drawResp.Results))
	}

	// 6. Test idempotency: reload presets should update without error or duplicates
	w = performRequest(router, "POST", "/api/admin/pool/load-presets", map[string]string{"file_path": "presets/characters.json"}, adminHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("Idempotent load-presets expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var totalChars int64
	DB.Model(&Character{}).Count(&totalChars)
	if totalChars != 10 {
		t.Fatalf("Expected 10 total characters in DB after reload, got %d", totalChars)
	}
}

func TestUpdatePoolConfigAPI(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	adminToken, err := GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("Failed to generate admin token: %v", err)
	}
	adminHeader := map[string]string{"Authorization": "Bearer " + adminToken}

	userToken, err := GenerateToken("user1", "user")
	if err != nil {
		t.Fatalf("Failed to generate user token: %v", err)
	}
	userHeader := map[string]string{"Authorization": "Bearer " + userToken}

	// 1. Non-admin user denied
	newS := 0.015
	payload := map[string]interface{}{
		"base_rate_s": newS,
	}
	w := performRequest(router, "PUT", "/api/admin/pool/config", payload, userHeader)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for non-admin, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Invalid validation checks
	invalidPayload := map[string]interface{}{
		"base_rate_s": 1.5, // > 1.0
	}
	w = performRequest(router, "PUT", "/api/admin/pool/config", invalidPayload, adminHeader)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for rate > 1, got %d: %s", w.Code, w.Body.String())
	}

	invalidPityPayload := map[string]interface{}{
		"soft_pity_start": 90,
		"hard_pity_s":     80, // soft_pity_start >= hard_pity_s
	}
	w = performRequest(router, "PUT", "/api/admin/pool/config", invalidPityPayload, adminHeader)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for soft_pity_start >= hard_pity_s, got %d: %s", w.Code, w.Body.String())
	}

	// Explicit invalid probability sum (> 1.0)
	invalidSumPayload := map[string]interface{}{
		"base_rate_s": 0.05,
		"base_rate_a": 0.15,
		"base_rate_b": 0.85, // sum = 1.05 > 1.0
	}
	w = performRequest(router, "PUT", "/api/admin/pool/config", invalidSumPayload, adminHeader)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for invalid probability sum > 1.0, got %d: %s", w.Code, w.Body.String())
	}

	// Negative auto-calculated B rate (< 0) when S + A > 1.0
	invalidNegativeBPayload := map[string]interface{}{
		"base_rate_s": 0.60,
		"base_rate_a": 0.50, // auto-calculated B = -0.10 < 0
	}
	w = performRequest(router, "PUT", "/api/admin/pool/config", invalidNegativeBPayload, adminHeader)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for negative auto-calculated B rate, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Valid update (auto-balancing B rate)
	validUpdate := map[string]interface{}{
		"base_rate_s":     0.012,
		"base_rate_a":     0.090,
		"hard_pity_s":     75,
		"soft_pity_start": 60,
		"soft_pity_inc":   0.060,
		"max_limited_s":   5,
	}
	w = performRequest(router, "PUT", "/api/admin/pool/config", validUpdate, adminHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid config update, got %d: %s", w.Code, w.Body.String())
	}

	// Verify atomic config updated (including auto-balanced BaseRateB)
	cfg := GetCurrentPoolConfig()
	if cfg.BaseRateS != 0.012 || cfg.BaseRateA != 0.090 || math.Abs(cfg.BaseRateB-0.898) > 1e-6 || cfg.HardPityS != 75 || cfg.MaxLimitedS != 5 {
		t.Fatalf("Global config atomic not updated properly: %+v", cfg)
	}

	// 4. Verify public GET /api/pool/info reflects updated config
	w = performRequest(router, "GET", "/api/pool/info", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for GET /api/pool/info, got %d", w.Code)
	}
	var infoResp struct {
		Config PoolConfig `json:"config"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &infoResp); err != nil {
		t.Fatalf("Failed to parse pool info: %v", err)
	}
	if infoResp.Config.BaseRateS != 0.012 || infoResp.Config.BaseRateA != 0.090 || math.Abs(infoResp.Config.BaseRateB-0.898) > 1e-6 || infoResp.Config.HardPityS != 75 {
		t.Fatalf("Pool info does not reflect updated config: %+v", infoResp.Config)
	}

	// 5. Valid update with explicit BaseRateB
	validExplicitB := map[string]interface{}{
		"base_rate_s": 0.020,
		"base_rate_a": 0.100,
		"base_rate_b": 0.880,
	}
	w = performRequest(router, "PUT", "/api/admin/pool/config", validExplicitB, adminHeader)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid explicit B rate, got %d: %s", w.Code, w.Body.String())
	}
	cfg = GetCurrentPoolConfig()
	if cfg.BaseRateS != 0.020 || cfg.BaseRateA != 0.100 || math.Abs(cfg.BaseRateB-0.880) > 1e-6 {
		t.Fatalf("Expected explicit BaseRateB to be 0.880, got %+v", cfg)
	}
}

func TestGRPCStreamingServerAndClient(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on tcp: %v", err)
	}
	addr := lis.Addr().String()

	grpcServer := grpc.NewServer()
	proto.RegisterConfigServiceServer(grpcServer, DefaultGRPCServer)
	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial gRPC server: %v", err)
	}
	defer conn.Close()

	client := proto.NewConfigServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.SubscribeConfigUpdates(ctx, &proto.EmptyRequest{})
	if err != nil {
		t.Fatalf("Failed to subscribe config updates: %v", err)
	}

	initMsg, err := stream.Recv()
	if err != nil {
		t.Fatalf("Failed to receive initial gRPC message: %v", err)
	}
	if initMsg.HardPityS == 0 {
		t.Fatalf("Expected non-zero HardPityS in initial message, got 0")
	}

	testCfg := PoolConfig{
		BaseRateS:     0.025,
		BaseRateA:     0.100,
		BaseRateB:     0.875,
		SoftPityStart: 50,
		SoftPityInc:   0.08,
		HardPityS:     70,
		HardPityA:     10,
		MaxLimitedS:   4,
	}
	BroadcastConfigToGRPC(&testCfg)

	updatedMsg, err := stream.Recv()
	if err != nil {
		t.Fatalf("Failed to receive updated gRPC message: %v", err)
	}
	if updatedMsg.BaseRateS != 0.025 || updatedMsg.HardPityS != 70 {
		t.Fatalf("Received unexpected gRPC update: S=%.4f, HardPity=%d", updatedMsg.BaseRateS, updatedMsg.HardPityS)
	}
}

func TestSSENotificationStream(t *testing.T) {
	ch := GlobalSSEBroker.Register()
	defer GlobalSSEBroker.Unregister(ch)

	GlobalSSEBroker.Broadcast("POOL_UPDATE", "角色【测试角色】已加入卡池！")

	select {
	case msg := <-ch:
		if msg.Event != "POOL_UPDATE" || msg.Data != "角色【测试角色】已加入卡池！" {
			t.Fatalf("Unexpected SSE message: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for POOL_UPDATE SSE message")
	}

	GlobalSSEBroker.Broadcast("PROB_UPDATE", "卡池概率配置已更新！")

	select {
	case msg := <-ch:
		if msg.Event != "PROB_UPDATE" || msg.Data != "卡池概率配置已更新！" {
			t.Fatalf("Unexpected SSE message: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for PROB_UPDATE SSE message")
	}
}

package tests

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func adminCharacterTestHeaders(t *testing.T) map[string]string {
	t.Helper()
	token, err := GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	return map[string]string{"Authorization": "Bearer " + token}
}

func setLimitedSCapForTest(t *testing.T, max int) {
	t.Helper()
	oldConfig := GetCurrentPoolConfig()
	t.Cleanup(func() { GlobalConfigAtomic.Store(&oldConfig) })
	newConfig := oldConfig
	newConfig.MaxLimitedS = max
	GlobalConfigAtomic.Store(&newConfig)
}

func TestCreateCharacterValidationAndAdminCatalog(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()
	headers := adminCharacterTestHeaders(t)

	created := performRequest(router, http.MethodPost, "/api/admin/character", map[string]interface{}{
		"name":       "  Test Character  ",
		"rarity":     " S ",
		"is_limited": true,
	}, headers)
	if created.Code != http.StatusOK {
		t.Fatalf("Create valid character status = %d, body = %s", created.Code, created.Body.String())
	}
	var createdResp struct {
		Data Character `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdResp); err != nil {
		t.Fatalf("Unmarshal create response: %v", err)
	}
	if createdResp.Data.Name != "Test Character" || createdResp.Data.Rarity != "S" || !createdResp.Data.IsLimited {
		t.Fatalf("Created character was not normalized: %+v", createdResp.Data)
	}

	for _, payload := range []map[string]interface{}{
		{"name": "   ", "rarity": "S"},
		{"name": "Limited A", "rarity": "A", "is_limited": true},
	} {
		response := performRequest(router, http.MethodPost, "/api/admin/character", payload, headers)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("Invalid character status = %d, want 400: %s", response.Code, response.Body.String())
		}
	}

	second := Character{Name: "Second", Rarity: "A"}
	if err := DB.Create(&second).Error; err != nil {
		t.Fatalf("Create second fixture: %v", err)
	}
	catalog := performRequest(router, http.MethodGet, "/api/admin/characters", nil, headers)
	if catalog.Code != http.StatusOK {
		t.Fatalf("Admin catalog status = %d, body = %s", catalog.Code, catalog.Body.String())
	}
	var catalogResp struct {
		Data []Character `json:"data"`
	}
	if err := json.Unmarshal(catalog.Body.Bytes(), &catalogResp); err != nil {
		t.Fatalf("Unmarshal catalog response: %v", err)
	}
	if len(catalogResp.Data) != 2 || catalogResp.Data[0].ID >= catalogResp.Data[1].ID {
		t.Fatalf("Catalog should include characters in ascending ID order, got %+v", catalogResp.Data)
	}

	unauthorized := performRequest(router, http.MethodGet, "/api/admin/characters", nil, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("Unauthenticated catalog status = %d, want 401", unauthorized.Code)
	}
}

func TestPushCharacterMaintainsLimitedSFIFOAndSingleUP(t *testing.T) {
	setupTestDB(t)
	setLimitedSCapForTest(t, 2)
	router := setupRouter()
	headers := adminCharacterTestHeaders(t)
	characters := []Character{
		{Name: "Oldest", Rarity: "S", IsLimited: true},
		{Name: "Middle", Rarity: "S", IsLimited: true},
		{Name: "Newest", Rarity: "S", IsLimited: true},
	}
	if err := DB.Create(&characters).Error; err != nil {
		t.Fatalf("Create limited S fixtures: %v", err)
	}
	push := func(id uint, isUp bool) *Character {
		t.Helper()
		response := performRequest(router, http.MethodPost, "/api/admin/pool/push", map[string]interface{}{
			"character_id": id,
			"is_up":        isUp,
		}, headers)
		if response.Code != http.StatusOK {
			t.Fatalf("Push character %d status = %d, body = %s", id, response.Code, response.Body.String())
		}
		var result struct {
			Data Character `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatalf("Unmarshal push response: %v", err)
		}
		return &result.Data
	}

	push(characters[0].ID, true)
	oldestEntry := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := DB.Model(&Character{}).Where("id = ?", characters[0].ID).Update("entered_pool_at", oldestEntry).Error; err != nil {
		t.Fatalf("Set oldest entry time: %v", err)
	}
	push(characters[1].ID, true)

	var afterUPSwitch Character
	if err := DB.First(&afterUPSwitch, characters[0].ID).Error; err != nil {
		t.Fatalf("Load oldest after UP switch: %v", err)
	}
	if !afterUPSwitch.IsInPool || afterUPSwitch.IsUp || afterUPSwitch.EnteredPoolAt == nil || !afterUPSwitch.EnteredPoolAt.Equal(oldestEntry) {
		t.Fatalf("UP switch should preserve pool membership and FIFO entry time: %+v", afterUPSwitch)
	}

	push(characters[2].ID, true)
	var got []Character
	if err := DB.Order("id ASC").Find(&got).Error; err != nil {
		t.Fatalf("Load characters after FIFO eviction: %v", err)
	}
	if len(got) != 3 || got[0].IsInPool || got[0].IsUp || !got[1].IsInPool || got[1].IsUp || !got[2].IsInPool || !got[2].IsUp {
		t.Fatalf("Expected oldest limited S evicted and one newest UP, got %+v", got)
	}
	var inPoolCount int64
	if err := DB.Model(&Character{}).Where("rarity = ? AND is_limited = ? AND is_in_pool = ?", "S", true, true).Count(&inPoolCount).Error; err != nil {
		t.Fatalf("Count limited S pool members: %v", err)
	}
	if inPoolCount != 2 {
		t.Fatalf("Limited S pool count = %d, want 2", inPoolCount)
	}
}

func TestPushOptionalRarityAndLimitedValidation(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()
	headers := adminCharacterTestHeaders(t)
	missing := performRequest(router, http.MethodPost, "/api/admin/pool/push", map[string]interface{}{
		"character_id": 99999,
	}, headers)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("Push missing character status = %d, want 404: %s", missing.Code, missing.Body.String())
	}

	character := Character{Name: "Override", Rarity: "S"}
	if err := DB.Create(&character).Error; err != nil {
		t.Fatalf("Create character: %v", err)
	}

	for _, rarity := range []string{"A", "B"} {
		invalidUP := performRequest(router, http.MethodPost, "/api/admin/pool/push", map[string]interface{}{
			"character_id": character.ID,
			"rarity":       rarity,
			"is_up":        true,
		}, headers)
		if invalidUP.Code != http.StatusBadRequest {
			t.Fatalf("Push %s rarity UP character status = %d, want 400: %s", rarity, invalidUP.Code, invalidUP.Body.String())
		}
	}
	var unchanged Character
	if err := DB.First(&unchanged, character.ID).Error; err != nil {
		t.Fatalf("Load character after invalid UP requests: %v", err)
	}
	if unchanged.Rarity != "S" || unchanged.IsInPool || unchanged.IsUp {
		t.Fatalf("Invalid non-S UP requests changed the character: %+v", unchanged)
	}

	invalid := performRequest(router, http.MethodPost, "/api/admin/pool/push", map[string]interface{}{
		"character_id": character.ID,
		"rarity":       "A",
		"is_limited":   true,
	}, headers)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("Push non-S limited character status = %d, want 400: %s", invalid.Code, invalid.Body.String())
	}

	valid := performRequest(router, http.MethodPost, "/api/admin/pool/push", map[string]interface{}{
		"character_id": character.ID,
		"rarity":       " S ",
		"is_limited":   true,
		"is_up":        true,
	}, headers)
	if valid.Code != http.StatusOK {
		t.Fatalf("Push with optional S flags status = %d, body = %s", valid.Code, valid.Body.String())
	}
	var stored Character
	if err := DB.First(&stored, character.ID).Error; err != nil {
		t.Fatalf("Load pushed character: %v", err)
	}
	if stored.Rarity != "S" || !stored.IsLimited || !stored.IsInPool || !stored.IsUp {
		t.Fatalf("Optional push fields were not applied: %+v", stored)
	}
}

func TestLoadPresetsValidatesSOnlyFlagsAndEvictsFIFO(t *testing.T) {
	setupTestDB(t)
	setLimitedSCapForTest(t, 1)
	router := setupRouter()
	headers := adminCharacterTestHeaders(t)
	writePreset := func(t *testing.T, body string) string {
		t.Helper()
		file, err := os.CreateTemp("presets", "admin-character-test-*.json")
		if err != nil {
			t.Fatalf("Create temporary preset: %v", err)
		}
		if _, err := file.WriteString(body); err != nil {
			file.Close()
			t.Fatalf("Write temporary preset: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("Close temporary preset: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(file.Name()) })
		return filepath.ToSlash(file.Name())
	}

	invalidPath := writePreset(t, `[{"name":"Invalid A","rarity":"A","is_limited":true}]`)
	invalid := performRequest(router, http.MethodPost, "/api/admin/pool/load-presets", map[string]string{"file_path": invalidPath}, headers)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("Load invalid non-S preset status = %d, want 400: %s", invalid.Code, invalid.Body.String())
	}

	validPath := writePreset(t, `[
		{"name":"Preset Old","rarity":"S","is_limited":true,"is_up":false},
		{"name":"Preset New","rarity":"S","is_limited":true,"is_up":true}
	]`)
	loaded := performRequest(router, http.MethodPost, "/api/admin/pool/load-presets", map[string]string{"file_path": validPath}, headers)
	if loaded.Code != http.StatusOK {
		t.Fatalf("Load valid presets status = %d, body = %s", loaded.Code, loaded.Body.String())
	}
	var got []Character
	if err := DB.Order("id ASC").Find(&got).Error; err != nil {
		t.Fatalf("Load preset characters: %v", err)
	}
	if len(got) != 2 || got[0].IsInPool || got[0].IsUp || !got[1].IsInPool || !got[1].IsUp {
		t.Fatalf("Expected FIFO eviction and one preset UP, got %+v", got)
	}
}

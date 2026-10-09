package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gacha-simulator/internal/database"
	"gacha-simulator/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type GithubProfile struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type OAuthProvider interface {
	GetAuthURL(state string) string
	ExchangeCode(code string) (string, error)
	GetUserProfile(token string) (*GithubProfile, error)
}

type GithubOAuthClient struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	HTTPClient   *http.Client
}

func NewGithubOAuthClient() *GithubOAuthClient {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	if clientID == "" {
		clientID = "Iv23liGWo2PtRjCjSTQ3"
	}
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = "cbb88685f1a5adbe01cd234760e8bcb7fc30b027"
	}
	redirectURI := os.Getenv("GITHUB_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/api/auth/github/callback"
	}

	return &GithubOAuthClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		HTTPClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (g *GithubOAuthClient) IsMock() bool {
	return g.ClientID == "mock_client_id" || os.Getenv("GITHUB_DEV_MOCK") == "true"
}

func (g *GithubOAuthClient) GetAuthURL(state string) string {
	if g.IsMock() {
		return fmt.Sprintf("%s?code=mock_code_dev&state=%s", g.RedirectURI, state)
	}
	params := url.Values{}
	params.Set("client_id", g.ClientID)
	params.Set("redirect_uri", g.RedirectURI)
	params.Set("scope", "read:user user:email")
	if state != "" {
		params.Set("state", state)
	}
	return "https://github.com/login/oauth/authorize?" + params.Encode()
}

func (g *GithubOAuthClient) ExchangeCode(code string) (string, error) {
	if g.IsMock() {
		return "mock_token_dev", nil
	}
	reqBody, err := json.Marshal(map[string]string{
		"client_id":     g.ClientID,
		"client_secret": g.ClientSecret,
		"code":          code,
		"redirect_uri":  g.RedirectURI,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := g.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github oauth returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp struct {
		AccessToken      string `json:"access_token"`
		TokenType        string `json:"token_type"`
		Scope            string `json:"scope"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", err
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("oauth error: %s (%s)", tokenResp.Error, tokenResp.ErrorDescription)
	}

	if tokenResp.AccessToken == "" {
		return "", errors.New("no access token in github response")
	}

	return tokenResp.AccessToken, nil
}

func (g *GithubOAuthClient) GetUserProfile(token string) (*GithubProfile, error) {
	if g.IsMock() {
		return &GithubProfile{
			ID:        999999,
			Login:     "github_mock_user",
			Name:      "Mock Octocat",
			Email:     "octocat_mock@github.com",
			AvatarURL: "https://github.com/ghost.png",
		}, nil
	}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := g.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var profile GithubProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

var CurrentOAuthProvider OAuthProvider = NewGithubOAuthClient()

func GithubLoginHandler(c *gin.Context) {
	state := c.Query("state")
	if state == "" {
		state = uuid.New().String()[:8]
	}
	authURL := CurrentOAuthProvider.GetAuthURL(state)
	if c.Query("redirect") == "true" {
		c.Redirect(http.StatusTemporaryRedirect, authURL)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state,
	})
}

func GithubCallbackHandler(c *gin.Context) {
	var req struct {
		Code  string `json:"code" form:"code"`
		State string `json:"state" form:"state"`
	}
	_ = c.ShouldBindQuery(&req)
	if req.Code == "" {
		_ = c.ShouldBindJSON(&req)
	}
	if req.Code == "" {
		req.Code = c.Query("code")
	}
	if req.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "authorization code is required"})
		return
	}

	accessToken, err := CurrentOAuthProvider.ExchangeCode(req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to exchange code: " + err.Error()})
		return
	}

	profile, err := CurrentOAuthProvider.GetUserProfile(accessToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get github profile: " + err.Error()})
		return
	}

	ghIDStr := strconv.FormatInt(profile.ID, 10)
	var user model.User
	err = database.DB.Where("github_id = ?", ghIDStr).First(&user).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error: " + err.Error()})
		return
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if profile.Email != "" {
			var existingEmailUser model.User
			if err := database.DB.Where("email = ?", profile.Email).First(&existingEmailUser).Error; err == nil {
				existingEmailUser.GithubID = &ghIDStr
				if err := database.DB.Save(&existingEmailUser).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to link github account: " + err.Error()})
					return
				}
				user = existingEmailUser
			}
		}

		if user.ID == "" {
			generatedID := uuid.New().String()[:8]
			nickname := profile.Login
			if nickname == "" {
				nickname = profile.Name
			}
			if nickname == "" {
				nickname = "gh_" + ghIDStr
			}
			hashedPwd, err := bcrypt.GenerateFromPassword([]byte(uuid.New().String()), bcrypt.DefaultCost)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
				return
			}
			var emailPtr *string
			if profile.Email != "" {
				emailPtr = &profile.Email
			}
			user = model.User{
				ID:       generatedID,
				Nickname: nickname,
				Password: string(hashedPwd),
				GithubID: &ghIDStr,
				Email:    emailPtr,
				Role:     "user",
			}
			if err := database.DB.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user: " + err.Error()})
				return
			}
		}
	}

	token, err := GenerateToken(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token: " + err.Error()})
		return
	}

	if strings.Contains(c.GetHeader("Accept"), "text/html") {
		userMap := gin.H{
			"id":        user.ID,
			"nickname":  user.Nickname,
			"role":      user.Role,
			"bio":       user.Bio,
			"email":     user.Email,
			"github_id": user.GithubID,
		}
		userJSONBytes, _ := json.Marshal(userMap)
		htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>GitHub Login</title></head>
<body>
<p style="text-align:center;margin-top:20vh;font-family:sans-serif;color:#333;">GitHub 授权成功，正在跳转...</p>
<script>
    localStorage.setItem('gacha_token', '%s');
    localStorage.setItem('gacha_user', JSON.stringify(%s));
    window.location.href = '/';
</script>
</body>
</html>`, token, string(userJSONBytes))
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(htmlContent))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "github login successfully",
		"token":   token,
		"user": gin.H{
			"id":        user.ID,
			"nickname":  user.Nickname,
			"role":      user.Role,
			"bio":       user.Bio,
			"email":     user.Email,
			"github_id": user.GithubID,
		},
	})
}

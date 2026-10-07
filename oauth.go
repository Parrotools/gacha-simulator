package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
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

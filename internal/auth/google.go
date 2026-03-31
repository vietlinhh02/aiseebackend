package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// TokenInfo chứa thông tin từ Google token verification
type TokenInfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Aud     string `json:"aud"` // Audience - Web Client ID
	Iss     string `json:"iss"` // Issuer
	Exp     string `json:"exp"` // Expiration
}

// GoogleUserInfo chứa thông tin user đã xác thực
type GoogleUserInfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// VerifyGoogleToken xác thực Google ID token với Google
// Nếu GOOGLE_WEB_CLIENT_ID được set, sẽ kiểm tra audience
func VerifyGoogleToken(ctx context.Context, idToken string) (*GoogleUserInfo, error) {
	url := "https://oauth2.googleapis.com/tokeninfo?id_token=" + idToken
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid token: status %d", resp.StatusCode)
	}

	var info TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Kiểm tra issuer
	if info.Iss != "https://accounts.google.com" && info.Iss != "accounts.google.com" {
		return nil, fmt.Errorf("invalid issuer: %s", info.Iss)
	}

	// Kiểm tra audience nếu có config
	expectedAud := os.Getenv("GOOGLE_WEB_CLIENT_ID")
	if expectedAud != "" && info.Aud != expectedAud {
		return nil, fmt.Errorf("invalid audience: expected %s, got %s", expectedAud, info.Aud)
	}

	return &GoogleUserInfo{
		Sub:     info.Sub,
		Email:   info.Email,
		Name:    info.Name,
		Picture: info.Picture,
	}, nil
}

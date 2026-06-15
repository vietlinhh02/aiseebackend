package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2/google"
)

type FCMSender struct {
	projectID          string
	serviceAccountPath string
	httpClient         *http.Client
	tokenSource        struct {
		sync.Mutex
		token   string
		expires time.Time
	}
}

func NewFCMSender(
	serviceAccountPath string,
) (*FCMSender, error) {
	data, err := os.ReadFile(serviceAccountPath)
	if err != nil {
		return nil, fmt.Errorf(
			"read service account: %w", err,
		)
	}

	var sa struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(data, &sa); err != nil {
		return nil, fmt.Errorf(
			"parse service account: %w", err,
		)
	}

	return &FCMSender{
		projectID:          sa.ProjectID,
		serviceAccountPath: serviceAccountPath,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (f *FCMSender) Name() string { return "fcm" }

func (f *FCMSender) getAccessToken(
	ctx context.Context,
) (string, error) {
	f.tokenSource.Lock()
	defer f.tokenSource.Unlock()

	if f.tokenSource.token != "" &&
		time.Now().Before(f.tokenSource.expires) {
		return f.tokenSource.token, nil
	}

	data, err := os.ReadFile(f.serviceAccountPath)
	if err != nil {
		return "", err
	}

	creds, err := google.CredentialsFromJSONWithTypeAndParams(
		ctx,
		data,
		google.ServiceAccount,
		google.CredentialsParams{
			Scopes: []string{
				"https://www.googleapis.com/auth/firebase.messaging",
			},
		},
	)
	if err != nil {
		return "", err
	}

	tok, err := creds.TokenSource.Token()
	if err != nil {
		return "", err
	}

	f.tokenSource.token = tok.AccessToken
	f.tokenSource.expires = tok.Expiry.Add(-time.Minute)
	return tok.AccessToken, nil
}

func (f *FCMSender) Send(
	ctx context.Context,
	token string,
	n Notification,
) error {
	accessToken, err := f.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	msg := map[string]any{
		"message": map[string]any{
			"token": token,
			"data":  n.Data,
		},
	}

	msgInner := msg["message"].(map[string]any)

	// For normal notifications, include notification block so the system tray
	// shows a notification when the app is in the background or killed.
	// For incoming calls (voip), use data-only so the Flutter app handles
	// display via flutter_callkit_incoming (the plugin draws the full-screen
	// CallKit UI on Android; iOS uses PushKit/CallKit).
	if n.Type != "voip" {
		msgInner["notification"] = map[string]string{
			"title": n.Title,
			"body":  n.Body,
		}
	}

	// Android config: high priority + 24h TTL keeps the message reachable
	// while still respecting FCM rate limits. Without TTL, messages can be
	// dropped if the device is offline for a long stretch.
	if n.Type == "voip" {
		msgInner["android"] = map[string]any{
			"priority": "HIGH",
			"ttl":      "86400s",
		}
	} else {
		// For normal messages, normal priority is enough. TTL 1h is plenty
		// for chat / reminder notifications.
		msgInner["android"] = map[string]any{
			"priority": "HIGH",
			"ttl":      "3600s",
		}
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	url := fmt.Sprintf(
		"https://fcm.googleapis.com/v1/projects/%s/messages:send",
		f.projectID,
	)
	req, err := http.NewRequestWithContext(
		ctx, "POST", url, bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set(
		"Authorization", "Bearer "+accessToken,
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fcm request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"fcm response: %d", resp.StatusCode,
		)
	}

	return nil
}

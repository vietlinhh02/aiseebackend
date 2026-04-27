package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/coder/websocket"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const defaultSessionTTL = 5 * time.Minute

const (
	MediaModeAudioPCM   = "audio_pcm"
	MediaModeCameraJPEG = "camera_jpeg"
)

type Service struct {
	jwtSecret  string
	sessionTTL time.Duration
	now        func() time.Time
	bridge     Bridge
}

func NewService(jwtSecret string) *Service {
	return NewServiceWithBridge(jwtSecret, NoopBridge{})
}

func NewServiceWithBridge(jwtSecret string, bridge Bridge) *Service {
	if bridge == nil {
		bridge = NoopBridge{}
	}

	return &Service{
		jwtSecret:  jwtSecret,
		sessionTTL: defaultSessionTTL,
		now:        time.Now,
		bridge:     bridge,
	}
}

func (s *Service) CreateSession(userID, role, wsURL string) (*SessionDescriptor, error) {
	if userID == "" {
		return nil, fmt.Errorf("missing user id")
	}

	sessionID := "ai-" + uuid.NewString()
	expiresAt := s.now().Add(s.sessionTTL).UTC()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   userID,
		"role":  role,
		"type":  "assistant_session",
		"scope": "ai_ws",
		"sid":   sessionID,
		"exp":   expiresAt.Unix(),
	})

	tokenStr, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	if wsURL == "" {
		return nil, fmt.Errorf("missing websocket url")
	}

	return &SessionDescriptor{
		SessionID: sessionID,
		WSURL:     wsURL + "?token=" + url.QueryEscape(tokenStr),
		AudioInput: AudioFormat{
			Encoding:   "pcm_s16le",
			SampleRate: 16000,
			Channels:   1,
		},
		AudioOutput: AudioFormat{
			Encoding:   "pcm_s16le",
			SampleRate: 24000,
			Channels:   1,
		},
		Camera: CameraFormat{
			Encoding: "jpeg",
			MaxFPS:   1,
		},
	}, nil
}

func (s *Service) ParseToken(tokenStr string) (Session, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(s.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return Session{}, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Session{}, fmt.Errorf("invalid claims")
	}

	userID, _ := claims["sub"].(string)
	if userID == "" {
		return Session{}, fmt.Errorf("missing subject")
	}

	tokenType, _ := claims["type"].(string)
	switch tokenType {
	case "", "access":
	case "assistant_session":
		scope, _ := claims["scope"].(string)
		if scope != "ai_ws" {
			return Session{}, fmt.Errorf("invalid session scope")
		}
	default:
		return Session{}, fmt.Errorf("unsupported token type")
	}

	role, _ := claims["role"].(string)
	sessionID, _ := claims["sid"].(string)
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	locale, _ := claims["locale"].(string)
	profile := claims["profile"]
	wakeGreeting := optionalBoolClaim(claims["wake_greeting"])

	return Session{
		SessionID:    sessionID,
		UserID:       userID,
		Role:         role,
		TokenType:    tokenType,
		MediaMode:    MediaModeAudioPCM,
		Locale:       locale,
		Profile:      profile,
		WakeGreeting: wakeGreeting,
	}, nil
}

func (s *Service) Proxy(ctx context.Context, session Session, clientConn *websocket.Conn) error {
	return s.bridge.Proxy(ctx, session, clientConn)
}

func ParseMediaMode(payload json.RawMessage) (string, error) {
	if len(payload) == 0 {
		return "", errMissingMediaMode
	}

	var request struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(payload, &request); err != nil {
		return "", err
	}

	switch request.Mode {
	case MediaModeAudioPCM, MediaModeCameraJPEG:
		return request.Mode, nil
	default:
		return "", errInvalidMediaMode
	}
}

func optionalBoolClaim(value any) *bool {
	v, ok := value.(bool)
	if !ok {
		return nil
	}

	return &v
}

package assistant

import (
	"context"
	"encoding/json"

	"github.com/coder/websocket"
)

type SessionDescriptor struct {
	SessionID   string       `json:"session_id"`
	WSURL       string       `json:"ws_url"`
	AudioInput  AudioFormat  `json:"audio_input"`
	AudioOutput AudioFormat  `json:"audio_output"`
	Camera      CameraFormat `json:"camera"`
}

type AudioFormat struct {
	Encoding   string `json:"encoding"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
}

type CameraFormat struct {
	Encoding string `json:"encoding"`
	MaxFPS   int    `json:"max_fps"`
}

type Session struct {
	SessionID    string
	UserID       string
	Role         string
	TokenType    string
	MediaMode    string
	Locale       string
	Profile      any
	WakeGreeting *bool
}

type ClientEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type ServerEnvelope struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type Bridge interface {
	Proxy(context.Context, Session, *websocket.Conn) error
}

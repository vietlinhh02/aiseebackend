package livekit

import (
	"time"

	"github.com/livekit/protocol/auth"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

type Client struct {
	apiKey    string
	apiSecret string
	host      string
}

func NewClient(host, apiKey, apiSecret string) *Client {
	return &Client{host: host, apiKey: apiKey, apiSecret: apiSecret}
}

func (c *Client) CreateToken(roomName, identity, name string) (string, error) {
	at := auth.NewAccessToken(c.apiKey, c.apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     roomName,
	}
	at.SetVideoGrant(grant).
		SetIdentity(identity).
		SetName(name).
		SetValidFor(time.Hour)

	return at.ToJWT()
}

func (c *Client) RoomClient() *lksdk.RoomServiceClient {
	return lksdk.NewRoomServiceClient(c.host, c.apiKey, c.apiSecret)
}

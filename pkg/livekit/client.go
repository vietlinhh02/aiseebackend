package livekit

import (
	"context"
	"fmt"
	"time"

	"github.com/livekit/protocol/auth"
	lkproto "github.com/livekit/protocol/livekit"
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
	canUpdate := true
	grant := &auth.VideoGrant{
		RoomJoin:             true,
		Room:                 roomName,
		CanUpdateOwnMetadata: &canUpdate,
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

func (c *Client) CreateRoom(
	ctx context.Context, roomName string,
) error {
	_, err := c.RoomClient().CreateRoom(
		ctx, &lkproto.CreateRoomRequest{Name: roomName},
	)
	if err != nil {
		return fmt.Errorf("create room %s: %w", roomName, err)
	}
	return nil
}

func (c *Client) DeleteRoom(
	ctx context.Context, roomName string,
) error {
	_, err := c.RoomClient().DeleteRoom(
		ctx, &lkproto.DeleteRoomRequest{Room: roomName},
	)
	if err != nil {
		return fmt.Errorf("delete room %s: %w", roomName, err)
	}
	return nil
}

func (c *Client) ListParticipants(
	ctx context.Context, roomName string,
) ([]*lkproto.ParticipantInfo, error) {
	resp, err := c.RoomClient().ListParticipants(
		ctx, &lkproto.ListParticipantsRequest{Room: roomName},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list participants %s: %w", roomName, err,
		)
	}
	return resp.Participants, nil
}

func (c *Client) CreateRoomWithTimeout(
	ctx context.Context, roomName string, emptyTimeoutSec uint32,
) error {
	_, err := c.RoomClient().CreateRoom(
		ctx, &lkproto.CreateRoomRequest{
			Name:         roomName,
			EmptyTimeout: emptyTimeoutSec,
		},
	)
	if err != nil {
		return fmt.Errorf("create room %s: %w", roomName, err)
	}
	return nil
}

func (c *Client) DispatchClient() *lksdk.AgentDispatchClient {
	return lksdk.NewAgentDispatchServiceClient(
		c.host, c.apiKey, c.apiSecret,
	)
}

func (c *Client) CreateRoomWithAgent(
	ctx context.Context, roomName string,
) error {
	// Always delete stale room to clear old auto-dispatch config.
	_ = c.DeleteRoom(ctx, roomName)

	_, err := c.RoomClient().CreateRoom(
		ctx, &lkproto.CreateRoomRequest{
			Name:             roomName,
			EmptyTimeout:     300,
			DepartureTimeout: 30,
		},
	)
	if err != nil {
		return fmt.Errorf("create room %s: %w", roomName, err)
	}

	_, err = c.DispatchClient().CreateDispatch(
		ctx, &lkproto.CreateAgentDispatchRequest{
			AgentName: "aisee-agent",
			Room:      roomName,
		},
	)
	if err != nil {
		return fmt.Errorf("dispatch agent to %s: %w", roomName, err)
	}

	return nil
}

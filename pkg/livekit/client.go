package livekit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/livekit/protocol/auth"
	lkproto "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

type Client struct {
	apiKey    string
	apiSecret string
	host      string
	roomSvc   *lksdk.RoomServiceClient
	dispSvc   *lksdk.AgentDispatchClient

	dispatches sync.Map
}

func NewClient(host, apiKey, apiSecret string) *Client {
	return &Client{
		host:      host,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		roomSvc:   lksdk.NewRoomServiceClient(host, apiKey, apiSecret),
		dispSvc:   lksdk.NewAgentDispatchServiceClient(host, apiKey, apiSecret),
	}
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
	return c.roomSvc
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
	return c.dispSvc
}

func (c *Client) CreateRoomWithAgent(
	ctx context.Context, roomName string,
) error {
	// 1. Debounce rapid retries (e.g., within 15 seconds) to prevent duplicate agent spawn while Booting
	if lastInter, ok := c.dispatches.Load(roomName); ok {
		if time.Since(lastInter.(time.Time)) < 15*time.Second {
			return nil // Recently dispatched, agent is likely booting
		}
	}

	// 2. Check if agent is already actively participating in the room
	participants, err := c.ListParticipants(ctx, roomName)
	if err == nil {
		for _, p := range participants {
			// Aisee agent connects with this specific name
			if p.Name == "aisee-agent" || strings.HasPrefix(p.Identity, "agent") {
				c.dispatches.Store(roomName, time.Now()) // Agent is here, update debounce
				return nil
			}
		}
	}

	// 3. Create room (updates settings if room already exists)
	_, err = c.RoomClient().CreateRoom(
		ctx, &lkproto.CreateRoomRequest{
			Name:             roomName,
			EmptyTimeout:     300,
			DepartureTimeout: 30,
		},
	)
	if err != nil {
		return fmt.Errorf("create room %s: %w", roomName, err)
	}

	// 4. Dispatch the new agent
	_, err = c.DispatchClient().CreateDispatch(
		ctx, &lkproto.CreateAgentDispatchRequest{
			AgentName: "aisee-agent",
			Room:      roomName,
		},
	)
	if err == nil {
		c.dispatches.Store(roomName, time.Now())
	} else {
		return fmt.Errorf("dispatch agent to %s: %w", roomName, err)
	}

	return nil
}

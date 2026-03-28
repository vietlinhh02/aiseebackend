package push

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
)

type APNsSender struct {
	client   *apns2.Client
	bundleID string
}

func NewAPNsSender(
	keyPath, keyID, teamID, bundleID string,
	production bool,
) (*APNsSender, error) {
	authKey, err := token.AuthKeyFromFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("load apns key: %w", err)
	}

	tok := &token.Token{
		AuthKey: authKey,
		KeyID:   keyID,
		TeamID:  teamID,
	}

	client := apns2.NewTokenClient(tok)
	if production {
		client = client.Production()
	} else {
		client = client.Development()
	}

	return &APNsSender{
		client: client, bundleID: bundleID,
	}, nil
}

func (a *APNsSender) Name() string { return "apns" }

func (a *APNsSender) Send(
	_ context.Context,
	deviceToken string,
	n Notification,
) error {
	payload := map[string]any{}

	if n.Type == "voip" {
		payload["aps"] = map[string]any{}
		for k, v := range n.Data {
			payload[k] = v
		}
	} else {
		payload["aps"] = map[string]any{
			"alert": map[string]string{
				"title": n.Title,
				"body":  n.Body,
			},
			"sound": "default",
		}
		if n.Data != nil {
			payload["data"] = n.Data
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	notification := &apns2.Notification{
		DeviceToken: deviceToken,
		Payload:     payloadBytes,
		Priority:    apns2.PriorityHigh,
	}

	if n.Type == "voip" {
		notification.Topic = a.bundleID + ".voip"
		notification.PushType = apns2.PushTypeVOIP
	} else {
		notification.Topic = a.bundleID
	}

	resp, err := a.client.Push(notification)
	if err != nil {
		return fmt.Errorf("apns push: %w", err)
	}

	if !resp.Sent() {
		return fmt.Errorf(
			"apns rejected: %s (%d)",
			resp.Reason, resp.StatusCode,
		)
	}

	return nil
}

package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"notifications/internal/notifications"
	"time"

	"gorm.io/datatypes"
)

type UserRegisteredHandler struct{}

func (UserRegisteredHandler) EventType() string {
	return "bank.user.registered"
}

func (UserRegisteredHandler) Build(event Envelope) ([]notifications.Notification, error) {
	if event.Version != 1 {
		return nil, fmt.Errorf("unsupported bank.user.registered version: %d", event.Version)
	}

	var data UserRegisteredV1
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return nil, fmt.Errorf("decode user registered v1: %w", err)
	}

	parsedEmail, err := mail.ParseAddress(data.Email)
	if data.UserID == "" || err != nil || parsedEmail.Address != data.Email {
		return nil, errors.New("invalid user registered payload")
	}

	payload, err := json.Marshal(map[string]string{"name": data.Name})
	if err != nil {
		return nil, fmt.Errorf("marshal notification payload: %w", err)
	}

	return []notifications.Notification{
		{
			EventID:     event.ID,
			UserID:      data.UserID,
			Recipient:   data.Email,
			Channel:     "email",
			Template:    "welcome_v1",
			Payload:     datatypes.JSON(payload),
			Status:      notifications.StatusPending,
			AvailableAt: time.Now().UTC(),
		},
	}, nil
}

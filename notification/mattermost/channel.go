package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/target/goalert/config"
	"github.com/target/goalert/notification"
	"github.com/target/goalert/notification/nfydest"
	"github.com/target/goalert/permission"
	"github.com/target/goalert/util/log"
	"github.com/target/goalert/validation"
)

type ChannelSender struct {
	cfg Config

	chanCache *ttlCache[string, *Channel]

	recv notification.Receiver
}

const (
	colorClosed  = "#218626"
	colorUnacked = "#862421"
	colorAcked   = "#867321"
)

var (
	_ nfydest.MessageSender       = &ChannelSender{}
	_ notification.ReceiverSetter = &ChannelSender{}
)

func NewChannelSender(ctx context.Context, cfg Config) (*ChannelSender, error) {
	if cfg.Client == nil {
		return nil, errors.New("http client is required")
	}

	return &ChannelSender{
		cfg:       cfg,
		chanCache: newTTLCache[string, *Channel](1000, 15*time.Minute),
	}, nil
}

func (s *ChannelSender) SetReceiver(r notification.Receiver) {
	s.recv = r
}

// Channel contains information about a Mattermost channel.
type Channel struct {
	ID        string
	Name      string
	TeamID    string
	WebhookURL string
}

func (c Channel) AsField() nfydest.FieldValue {
	return nfydest.FieldValue{
		Value: c.ID,
		Label: c.Name,
	}
}

// MattermostMessage represents a message to be sent to Mattermost
type MattermostMessage struct {
	Channel     string                      `json:"channel,omitempty"`
	Username    string                      `json:"username,omitempty"`
	IconURL     string                      `json:"icon_url,omitempty"`
	Text        string                      `json:"text"`
	Attachments []MattermostMessageAttachment `json:"attachments,omitempty"`
}

// MattermostMessageAttachment represents a message attachment
type MattermostMessageAttachment struct {
	Color   string                         `json:"color,omitempty"`
	Text    string                         `json:"text"`
	Actions []MattermostMessageAction      `json:"actions,omitempty"`
}

// MattermostMessageAction represents an interactive action button
type MattermostMessageAction struct {
	ID          string                           `json:"id"`
	Name        string                           `json:"name"`
	Integration MattermostMessageActionIntegration `json:"integration"`
	Type        string                           `json:"type,omitempty"`
	Style       string                           `json:"style,omitempty"`
}

// MattermostMessageActionIntegration contains the integration URL and context
type MattermostMessageActionIntegration struct {
	URL     string                 `json:"url"`
	Context map[string]interface{} `json:"context"`
}

func (s *ChannelSender) ValidateChannel(ctx context.Context, id string) error {
	err := permission.LimitCheckAny(ctx, permission.User, permission.System)
	if err != nil {
		return err
	}

	if id == "" {
		return validation.NewGenericError("Channel webhook URL is required.")
	}

	// Parse and validate webhook URL
	u, err := url.Parse(id)
	if err != nil {
		return validation.NewGenericError("Invalid webhook URL format.")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return validation.NewGenericError("Webhook URL must use http or https.")
	}

	return nil
}

// Channel will lookup a single Mattermost channel by webhook URL
func (s *ChannelSender) Channel(ctx context.Context, webhookURL string) (*Channel, error) {
	err := permission.LimitCheckAny(ctx, permission.User, permission.System)
	if err != nil {
		return nil, err
	}

	// For Mattermost, the webhook URL is the identifier
	// We'll cache basic channel info
	res, ok := s.chanCache.Get(webhookURL)
	if ok {
		return res, nil
	}

	// Create a basic channel object
	ch := &Channel{
		ID:        webhookURL,
		Name:      "Mattermost Channel",
		WebhookURL: webhookURL,
	}

	s.chanCache.Add(webhookURL, ch)
	return ch, nil
}

func alertLink(ctx context.Context, alertID int, summary string) string {
	cfg := config.FromContext(ctx)
	return fmt.Sprintf("[Alert #%d](%s): %s", alertID, cfg.CallbackURL("/alerts/"+fmt.Sprint(alertID)), summary)
}

func (s *ChannelSender) onCallNotificationText(ctx context.Context, msg notification.ScheduleOnCallUsers) string {
	cfg := config.FromContext(ctx)
	schedURL := cfg.CallbackURL("/schedules/" + msg.ScheduleID)

	var buf strings.Builder
	fmt.Fprintf(&buf, "**On-Call Notification**\n\n")
	fmt.Fprintf(&buf, "[Schedule %s](%s)\n\n", msg.ScheduleName, schedURL)

	if len(msg.Users) == 0 {
		fmt.Fprintf(&buf, "No one is currently on-call.\n")
	} else {
		fmt.Fprintf(&buf, "Currently on-call:\n")
		for _, u := range msg.Users {
			fmt.Fprintf(&buf, "- %s\n", u.Name)
		}
	}

	return buf.String()
}

func (s *ChannelSender) buildAlertMessage(ctx context.Context, msgID string, alertID int, summary, logEntry string, state notification.AlertState) MattermostMessage {
	cfg := config.FromContext(ctx)

	var color string
	switch state {
	case notification.AlertStateUnacknowledged:
		color = colorUnacked
	case notification.AlertStateAcknowledged:
		color = colorAcked
	case notification.AlertStateClosed:
		color = colorClosed
	}

	attachment := MattermostMessageAttachment{
		Color: color,
		Text:  fmt.Sprintf("**%s**\n\n%s", alertLink(ctx, alertID, summary), logEntry),
	}

	// Add interactive actions if enabled
	if cfg.Mattermost.InteractiveMessages {
		baseURL := cfg.CallbackURL("/api/v2/mattermost/message-action")

		var actions []MattermostMessageAction
		switch state {
		case notification.AlertStateUnacknowledged:
			actions = []MattermostMessageAction{
				{
					ID:   "ack",
					Name: "Acknowledge",
					Type: "button",
					Style: "primary",
					Integration: MattermostMessageActionIntegration{
						URL: baseURL,
						Context: map[string]interface{}{
							"message_id": msgID,
							"alert_id":   alertID,
							"action":     "ack",
						},
					},
				},
				{
					ID:   "close",
					Name: "Close",
					Type: "button",
					Style: "good",
					Integration: MattermostMessageActionIntegration{
						URL: baseURL,
						Context: map[string]interface{}{
							"message_id": msgID,
							"alert_id":   alertID,
							"action":     "close",
						},
					},
				},
			}
		case notification.AlertStateAcknowledged:
			actions = []MattermostMessageAction{
				{
					ID:   "close",
					Name: "Close",
					Type: "button",
					Style: "good",
					Integration: MattermostMessageActionIntegration{
						URL: baseURL,
						Context: map[string]interface{}{
							"message_id": msgID,
							"alert_id":   alertID,
							"action":     "close",
						},
					},
				},
			}
		}

		if len(actions) > 0 {
			attachment.Actions = actions
		}
	}

	return MattermostMessage{
		Username: cfg.ApplicationName(),
		Text:     "",
		Attachments: []MattermostMessageAttachment{attachment},
	}
}

func (s *ChannelSender) sendWebhook(ctx context.Context, webhookURL string, msg MattermostMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.cfg.Client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (s *ChannelSender) SendMessage(ctx context.Context, msg notification.Message) (*notification.SentMessage, error) {
	cfg := config.FromContext(ctx)

	webhookURL := msg.DestArg(FieldMattermostChannelID)
	userID := msg.DestArg(FieldMattermostUserID)

	// For DM, use the DM webhook URL if provided
	if userID != "" {
		dmWebhookURL := msg.DestArg(FieldMattermostDMWebhookURL)
		if dmWebhookURL != "" {
			webhookURL = dmWebhookURL
		} else if s.cfg.DMWebhookURL != "" {
			// Fallback to config DM webhook URL if available
			webhookURL = s.cfg.DMWebhookURL
		}
	}

	if webhookURL == "" {
		return nil, errors.New("webhook URL is required")
	}

	var mmMsg MattermostMessage

	switch t := msg.(type) {
	case notification.Test:
		mmMsg = MattermostMessage{
			Username: cfg.ApplicationName(),
			Text:     "This is a test message.",
		}
	case notification.Verification:
		mmMsg = MattermostMessage{
			Username: cfg.ApplicationName(),
			Text:     fmt.Sprintf("Your verification code is: `%s`", t.Code),
		}
	case notification.Alert:
		if t.OriginalStatus != nil {
			// Reply in thread if we already sent a message for this alert
			mmMsg = MattermostMessage{
				Username: cfg.ApplicationName(),
				Text:     alertLink(ctx, t.AlertID, t.Summary),
			}
		} else {
			mmMsg = s.buildAlertMessage(ctx, t.MsgID(), t.AlertID, t.Summary, "Unacknowledged", notification.AlertStateUnacknowledged)
		}
	case notification.AlertStatus:
		// For status updates, we can't actually update the original message in Mattermost
		// incoming webhooks, so we'll post a new message
		mmMsg = s.buildAlertMessage(ctx, t.OriginalStatus.ID, t.AlertID, t.Summary, t.LogEntry, t.NewAlertState)
	case notification.AlertBundle:
		mmMsg = MattermostMessage{
			Username: cfg.ApplicationName(),
			Text:     fmt.Sprintf("**Service '%s' has %d unacknowledged alerts**\n\n[View Alerts](%s)", t.ServiceName, t.Count, cfg.CallbackURL("/services/"+t.ServiceID+"/alerts")),
		}
	case notification.SignalMessage:
		mmMsg = MattermostMessage{
			Username: cfg.ApplicationName(),
			Text:     t.Param("message"),
		}
	case notification.ScheduleOnCallUsers:
		mmMsg = MattermostMessage{
			Username: cfg.ApplicationName(),
			Text:     s.onCallNotificationText(ctx, t),
		}
	default:
		return nil, errors.Errorf("unsupported message type: %T", t)
	}

	// If userID is provided, override the channel to send as DM
	if userID != "" {
		mmMsg.Channel = "@" + userID
	}

	err := s.sendWebhook(ctx, webhookURL, mmMsg)
	if err != nil {
		log.Log(ctx, fmt.Errorf("send mattermost webhook: %w", err))
		return nil, err
	}

	// Mattermost incoming webhooks don't return a message ID from the API.
	// We use our own message ID as the external ID to prevent duplicate sends
	// while acknowledging we can't update or track the actual posted message.
	return &notification.SentMessage{
		ExternalID: msg.MsgID(),
		State:      notification.StateDelivered,
	}, nil
}

// Status checks the status of the Mattermost sender
func (s *ChannelSender) Status(ctx context.Context) error {
	return nil
}

package mattermost

import (
	"context"

	"github.com/target/goalert/config"
	"github.com/target/goalert/notification/nfydest"
	"github.com/target/goalert/validation"
)

var (
	_ nfydest.Provider = (*ChannelSender)(nil)
)

func (s *ChannelSender) ID() string { return DestTypeMattermostChannel }

func (s *ChannelSender) TypeInfo(ctx context.Context) (*nfydest.TypeInfo, error) {
	cfg := config.FromContext(ctx)

	return &nfydest.TypeInfo{
		Type:                       DestTypeMattermostChannel,
		Name:                       "Mattermost Channel",
		Enabled:                    cfg.Mattermost.Enable,
		SupportsAlertNotifications: true,
		SupportsStatusUpdates:      true,
		SupportsOnCallNotify:       true,
		StatusUpdatesRequired:      true,
		SupportsSignals:            true,
		RequiredFields: []nfydest.FieldConfig{{
			FieldID:        FieldMattermostChannelID,
			Label:          "Mattermost Incoming Webhook URL",
			InputType:      "text",
			SupportsSearch: false,
			Hint:           "Enter the incoming webhook URL from your Mattermost channel integration settings (e.g., https://your-mattermost-server.com/hooks/xxx-generatedkey-xxx).",
		}},
		DynamicParams: []nfydest.DynamicParamConfig{{
			ParamID: "message",
			Label:   "Message",
			Hint:    "The text of the message to send.",
		}},
	}, nil
}

func (s *ChannelSender) ValidateField(ctx context.Context, fieldID, value string) error {
	switch fieldID {
	case FieldMattermostChannelID:
		return s.ValidateChannel(ctx, value)
	}

	return validation.NewGenericError("unknown field ID")
}

func (s *ChannelSender) DisplayInfo(ctx context.Context, args map[string]string) (*nfydest.DisplayInfo, error) {
	if args == nil {
		args = make(map[string]string)
	}

	webhookURL := args[FieldMattermostChannelID]
	if webhookURL == "" {
		return nil, validation.NewGenericError("webhook URL is required")
	}

	return &nfydest.DisplayInfo{
		IconURL:     FallbackIconURL,
		IconAltText: "Mattermost",
		LinkURL:     "",
		Text:        "Mattermost Channel",
	}, nil
}

func (s *ChannelSender) FieldLabel(ctx context.Context, fieldID, value string) (string, error) {
	switch fieldID {
	case FieldMattermostChannelID:
		return "Mattermost Channel", nil
	}

	return "", validation.NewGenericError("unknown field ID")
}

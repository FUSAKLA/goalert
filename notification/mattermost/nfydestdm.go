package mattermost

import (
	"context"
	"strings"

	"github.com/target/goalert/config"
	"github.com/target/goalert/notification/nfydest"
	"github.com/target/goalert/validation"
)

var _ nfydest.Provider = (*DMSender)(nil)

func (dm *DMSender) ID() string { return DestTypeMattermostDirectMessage }

func (dm *DMSender) TypeInfo(ctx context.Context) (*nfydest.TypeInfo, error) {
	cfg := config.FromContext(ctx)
	return &nfydest.TypeInfo{
		Type:                       DestTypeMattermostDirectMessage,
		Name:                       "Mattermost Message (DM)",
		Enabled:                    cfg.Mattermost.Enable,
		SupportsAlertNotifications: true,
		SupportsUserVerification:   true,
		SupportsStatusUpdates:      false,
		UserVerificationRequired:   true,
		StatusUpdatesRequired:      false,
		RequiredFields: []nfydest.FieldConfig{
			{
				FieldID:         FieldMattermostDMWebhookURL,
				Label:           "Mattermost Incoming Webhook URL",
				PlaceholderText: "https://your-mattermost.com/hooks/xxx",
				InputType:       "text",
				Hint:            `Create a DM with the GoAlert bot, then go to Main Menu > Integrations > Incoming Webhooks to create a webhook for that DM channel.`,
			},
			{
				FieldID:         FieldMattermostUserID,
				Label:           "Mattermost Username",
				PlaceholderText: "username",
				InputType:       "text",
				Hint:            `Enter your Mattermost username (without the @ symbol).`,
			},
		},
	}, nil
}

func (dm *DMSender) ValidateField(ctx context.Context, fieldID, value string) error {
	switch fieldID {
	case FieldMattermostDMWebhookURL:
		if value == "" {
			return validation.NewFieldError(FieldMattermostDMWebhookURL, "Webhook URL is required")
		}
		// Basic URL validation
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			return validation.NewFieldError(FieldMattermostDMWebhookURL, "Webhook URL must start with http:// or https://")
		}
		return nil
	case FieldMattermostUserID:
		// Basic validation - ensure it's not empty
		if value == "" {
			return validation.NewFieldError(FieldMattermostUserID, "Username is required")
		}
		return nil
	}

	return validation.NewGenericError("unknown field ID")
}

func (dm *DMSender) DisplayInfo(ctx context.Context, args map[string]string) (*nfydest.DisplayInfo, error) {
	if args == nil {
		args = make(map[string]string)
	}

	userID := args[FieldMattermostUserID]
	if userID == "" {
		return nil, validation.NewFieldError(FieldMattermostUserID, "Username is required")
	}

	return &nfydest.DisplayInfo{
		IconURL:     FallbackIconURL,
		IconAltText: "Mattermost",
		LinkURL:     "",
		Text:        "@" + userID,
	}, nil
}

func (dm *DMSender) FieldLabel(ctx context.Context, fieldID, value string) (string, error) {
	switch fieldID {
	case FieldMattermostUserID:
		if value == "" {
			return "", validation.NewFieldError(FieldMattermostUserID, "Username is required")
		}
		return "@" + value, nil
	}

	return "", validation.NewGenericError("unknown field ID")
}

package mattermost

import (
	"net/http"

	"github.com/target/goalert/user"
)

// Config contains values used for the Mattermost notification sender.
type Config struct {
	BaseURL   string
	UserStore *user.Store
	Client    *http.Client

	// DMWebhookURL is the webhook URL template for DMs.
	// The placeholder @{username} will be replaced with the actual username.
	DMWebhookURL string
}

package app

import (
	"context"

	"github.com/target/goalert/notification/mattermost"
)

func (app *App) initMattermost(ctx context.Context) error {
	var err error
	app.mattermostChan, err = mattermost.NewChannelSender(ctx, mattermost.Config{
		BaseURL:      app.cfg.MattermostBaseURL,
		UserStore:    app.UserStore,
		Client:       app.httpClient,
		DMWebhookURL: app.cfg.MattermostDMWebhookURL,
	})
	if err != nil {
		return err
	}

	return nil
}

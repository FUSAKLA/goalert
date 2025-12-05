package mattermost

import "github.com/target/goalert/gadb"

const (
	DestTypeMattermostChannel       = "builtin-mattermost-channel"
	DestTypeMattermostDirectMessage = "builtin-mattermost-dm"

	FieldMattermostChannelID    = "mattermost_channel_id"
	FieldMattermostUserID       = "mattermost_user_id"
	FieldMattermostDMWebhookURL = "mattermost_dm_webhook_url"

	FallbackIconURL = "builtin://mattermost"
)

func NewChannelDest(id string) gadb.DestV1 {
	return gadb.NewDestV1(DestTypeMattermostChannel, FieldMattermostChannelID, id)
}

func NewDirectMessageDest(userID string) gadb.DestV1 {
	return gadb.NewDestV1(DestTypeMattermostDirectMessage, FieldMattermostUserID, userID)
}

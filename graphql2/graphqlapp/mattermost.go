package graphqlapp

import (
	"context"

	"github.com/target/goalert/graphql2"
	"github.com/target/goalert/notification/mattermost"
	"github.com/target/goalert/search"
)

func (q *Query) MattermostChannel(ctx context.Context, id string) (*mattermost.Channel, error) {
	return q.MattermostStore.Channel(ctx, id)
}

// MattermostChannels is a GraphQL resolver for a list of Mattermost channels.
func (q *Query) MattermostChannels(ctx context.Context, input *graphql2.MattermostChannelSearchOptions) (conn *graphql2.MattermostChannelConnection, err error) {
	if input == nil {
		input = &graphql2.MattermostChannelSearchOptions{}
	}

	var searchOpts struct {
		Search string   `json:"s,omitempty"`
		Omit   []string `json:"m,omitempty"`
		After  struct {
			Name string `json:"n,omitempty"`
		} `json:"a,omitempty"`
	}
	searchOpts.Omit = input.Omit
	if input.Search != nil {
		searchOpts.Search = *input.Search
	}
	if input.After != nil && *input.After != "" {
		err = search.ParseCursor(*input.After, &searchOpts)
		if err != nil {
			return nil, err
		}
	}

	// For Mattermost, we don't have a way to list channels
	// since we're using incoming webhooks. Return empty list.
	conn = new(graphql2.MattermostChannelConnection)
	conn.PageInfo = &graphql2.PageInfo{}
	conn.Nodes = []mattermost.Channel{}

	return conn, nil
}

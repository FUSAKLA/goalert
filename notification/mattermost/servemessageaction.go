package mattermost

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/target/goalert/alert"
	"github.com/target/goalert/auth/authlink"
	"github.com/target/goalert/config"
	"github.com/target/goalert/notification"
	"github.com/target/goalert/util/errutil"
	"github.com/target/goalert/util/log"
	"github.com/target/goalert/validation"
)

// MattermostActionRequest represents the payload from Mattermost interactive messages
type MattermostActionRequest struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	ChannelID string `json:"channel_id"`
	TeamID    string `json:"team_id"`
	PostID    string `json:"post_id"`
	Context   struct {
		MessageID string `json:"message_id"`
		AlertID   int    `json:"alert_id"`
		Action    string `json:"action"`
	} `json:"context"`
}

// MattermostActionResponse represents the response to an interactive message action
type MattermostActionResponse struct {
	Update        *MattermostUpdateMessage `json:"update,omitempty"`
	EphemeralText string                   `json:"ephemeral_text,omitempty"`
}

// MattermostUpdateMessage represents an update to the original message
type MattermostUpdateMessage struct {
	Message string                 `json:"message"`
	Props   map[string]interface{} `json:"props,omitempty"`
}

func (s *ChannelSender) ServeMessageAction(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	cfg := config.FromContext(ctx)

	if !cfg.Mattermost.InteractiveMessages {
		http.Error(w, "not enabled", http.StatusNotFound)
		return
	}

	var payload MattermostActionRequest
	err := json.NewDecoder(req.Body).Decode(&payload)
	if errutil.HTTPError(ctx, w, err) {
		return
	}

	if payload.Context.Action == "" {
		errutil.HTTPError(ctx, w, validation.NewFieldError("context.action", "action is required"))
		return
	}

	var res notification.Result
	switch payload.Context.Action {
	case "ack":
		res = notification.ResultAcknowledge
	case "close":
		res = notification.ResultResolve
	default:
		errutil.HTTPError(ctx, w, validation.NewFieldErrorf("context.action", "unknown action '%s'", payload.Context.Action))
		return
	}

	var e *notification.UnknownSubjectError
	err = s.recv.ReceiveSubject(ctx, "mattermost:"+payload.TeamID, payload.UserID, payload.Context.MessageID, res)

	if errors.As(err, &e) {
		var linkURL string
		switch {
		case payload.UserID == "", payload.TeamID == "":
			// missing data, don't allow linking
			log.Log(ctx, errors.New("mattermost payload missing required data"))
		default:
			linkURL, err = s.recv.AuthLinkURL(ctx, "mattermost:"+payload.TeamID, payload.UserID, authlink.Metadata{
				UserDetails: fmt.Sprintf("Mattermost user %s", payload.UserName),
				AlertID:     e.AlertID,
				AlertAction: res.String(),
			})
			if err != nil {
				log.Log(ctx, err)
			}
		}

		var resp MattermostActionResponse
		if linkURL == "" {
			resp.EphemeralText = "Your Mattermost account isn't currently linked to GoAlert, please try again later."
		} else {
			resp.EphemeralText = fmt.Sprintf("Please link your Mattermost account with GoAlert: %s", linkURL)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	if alert.IsAlreadyAcknowledged(err) || alert.IsAlreadyClosed(err) {
		// ignore errors from duplicate requests
		var resp MattermostActionResponse
		resp.EphemeralText = "Alert has already been updated."
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	if errutil.HTTPError(ctx, w, err) {
		return
	}

	// Send success response
	var resp MattermostActionResponse
	switch res {
	case notification.ResultAcknowledge:
		resp.EphemeralText = fmt.Sprintf("Alert #%d acknowledged.", payload.Context.AlertID)
	case notification.ResultResolve:
		resp.EphemeralText = fmt.Sprintf("Alert #%d closed.", payload.Context.AlertID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

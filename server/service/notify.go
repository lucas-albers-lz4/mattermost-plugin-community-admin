package service

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

// ApplyPushDefaults sets mobile push notification preferences for a user.
func ApplyPushDefaults(client *pluginapi.Client, user *model.User) error {
	if user.NotifyProps == nil {
		user.NotifyProps = make(map[string]string)
	}
	user.NotifyProps["push"] = "all"
	user.NotifyProps["push_status"] = "online"
	return client.User.Update(user)
}

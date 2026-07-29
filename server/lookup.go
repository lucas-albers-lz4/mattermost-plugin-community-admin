package main

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"

	"github.com/lalbers/mattermost-plugin-community-admin/server/authz"
)

func newPluginUserLookup(client *pluginapi.Client) authz.UserLookup {
	return authz.NewPluginUserLookup(client)
}

func sanitizeUser(u *model.User) map[string]any {
	return map[string]any{
		"id":         u.Id,
		"username":   u.Username,
		"first_name": u.FirstName,
		"last_name":  u.LastName,
		"email":      u.Email,
		"delete_at":  u.DeleteAt,
		"roles":      u.Roles,
	}
}

func actorUsername(client *pluginapi.Client, actorID string) string {
	user, err := client.User.Get(actorID)
	if err != nil {
		return ""
	}
	return user.Username
}

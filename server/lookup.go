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
		"delete_at":  u.DeleteAt,
	}
}

func sanitizeUserWithTeams(u *model.User, teamIDs []string) map[string]any {
	out := sanitizeUser(u)
	out["team_ids"] = teamIDs
	return out
}

func actorUsername(client *pluginapi.Client, actorID string) string {
	if client == nil {
		return actorID
	}
	user, err := client.User.Get(actorID)
	if err != nil {
		client.Log.Warn("actor username lookup failed; using actor id", "actor_id", actorID, "error", err.Error())
		return actorID
	}
	return user.Username
}

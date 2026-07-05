package main

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"

	"github.com/lalbers/mattermost-plugin-community-admin/server/authz"
)

// pluginUserLookup adapts plugin API to authz.UserLookup.
type pluginUserLookup struct {
	client *pluginapi.Client
}

func newPluginUserLookup(client *pluginapi.Client) authz.UserLookup {
	return &pluginUserLookup{client: client}
}

func (l *pluginUserLookup) GetUserInfo(userID string) (*authz.UserInfo, error) {
	user, err := l.client.User.Get(userID)
	if err != nil {
		return nil, err
	}
	teams, err := l.client.Team.List(pluginapi.FilterTeamsByUser(userID))
	if err != nil {
		return nil, err
	}
	teamIDs := make([]string, 0, len(teams))
	for _, t := range teams {
		teamIDs = append(teamIDs, t.Id)
	}
	return &authz.UserInfo{
		ID:       user.Id,
		Username: user.Username,
		Roles:    user.Roles,
		IsBot:    user.IsBot,
		TeamIDs:  teamIDs,
	}, nil
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

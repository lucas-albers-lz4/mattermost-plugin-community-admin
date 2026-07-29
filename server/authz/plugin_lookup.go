package authz

import (
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

// PluginUserLookup adapts plugin API to UserLookup.
type PluginUserLookup struct {
	Client *pluginapi.Client
}

func NewPluginUserLookup(client *pluginapi.Client) *PluginUserLookup {
	return &PluginUserLookup{Client: client}
}

func (l *PluginUserLookup) GetUserInfo(userID string) (*UserInfo, error) {
	user, err := l.Client.User.Get(userID)
	if err != nil {
		return nil, err
	}
	teams, err := l.Client.Team.List(pluginapi.FilterTeamsByUser(userID))
	if err != nil {
		return nil, err
	}
	teamIDs := make([]string, 0, len(teams))
	for _, t := range teams {
		teamIDs = append(teamIDs, t.Id)
	}
	return &UserInfo{
		ID:       user.Id,
		Username: user.Username,
		Roles:    user.Roles,
		IsBot:    user.IsBot,
		TeamIDs:  teamIDs,
	}, nil
}

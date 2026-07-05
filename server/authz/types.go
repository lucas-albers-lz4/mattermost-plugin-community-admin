package authz

import "github.com/lalbers/mattermost-plugin-community-admin/server/config"

// Operation identifies a plugin action for authorization checks.
type Operation string

const (
	OpListUsers           Operation = "list_users"
	OpCreateUser          Operation = "create_user"
	OpEditProfile         Operation = "edit_profile"
	OpResetPassword       Operation = "reset_password"
	OpAddTeamMember       Operation = "add_team_member"
	OpRemoveTeamMember    Operation = "remove_team_member"
	OpAddChannelMember    Operation = "add_channel_member"
	OpRemoveChannelMember Operation = "remove_channel_member"
	OpDeactivateGlobal    Operation = "deactivate_global"
	OpReactivate          Operation = "reactivate"
	OpBatchImport         Operation = "batch_import"
	OpViewAudit           Operation = "view_audit"
)

// Target describes the resource being acted upon.
type Target struct {
	UserID    string
	TeamID    string
	ChannelID string
}

// UserInfo is the minimal user data needed for authorization.
type UserInfo struct {
	ID       string
	Username string
	Roles    string
	IsBot    bool
	TeamIDs  []string
}

// UserLookup resolves users for authorization checks.
type UserLookup interface {
	GetUserInfo(userID string) (*UserInfo, error)
}

// OrganizerContext is the resolved organizer scope for an authenticated actor.
type OrganizerContext struct {
	ActorID   string
	Organizer *config.Organizer
	Config    *config.ScopeConfig
}

package authz

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
)

type mockLookup struct {
	users map[string]*UserInfo
}

func (m *mockLookup) GetUserInfo(userID string) (*UserInfo, error) {
	u, ok := m.users[userID]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func testConfig() *config.ScopeConfig {
	return &config.ScopeConfig{
		Version:     1,
		EmailDomain: "community.local",
		SiteURL:     "https://chat.example.com",
		Organizers: []config.Organizer{
			{
				UserID:          "organizer-a",
				DisplayUsername: "coach.smith",
				Teams:           []config.TeamRef{{ID: "team-soccer", Name: "u12-soccer"}},
				Channels:        []config.ChannelRef{{ID: "chan-chat", TeamID: "team-soccer", Name: "team-chat"}},
				Permissions: config.Permissions{
					CreateUser:         true,
					EditProfile:        true,
					ResetPassword:      true,
					ManageMembership:   true,
					RemoveFromTeam:     true,
					DeactivateGlobally: false,
				},
			},
			{
				UserID:          "organizer-b",
				DisplayUsername: "jane.lead",
				Teams: []config.TeamRef{
					{ID: "team-soccer", Name: "u12-soccer"},
					{ID: "team-parents", Name: "parents"},
				},
				AllChannelsInTeams: []string{"team-soccer", "team-parents"},
				Permissions: config.Permissions{
					CreateUser:         true,
					EditProfile:        true,
					ResetPassword:      true,
					ManageMembership:   true,
					RemoveFromTeam:     true,
					DeactivateGlobally: true,
				},
			},
			{
				UserID:          "organizer-c",
				DisplayUsername: "coach.limited",
				Teams:           []config.TeamRef{{ID: "team-soccer", Name: "u12-soccer"}},
				Permissions: config.Permissions{
					CreateUser:         true,
					EditProfile:        true,
					ResetPassword:      true,
					ManageMembership:   true,
					RemoveFromTeam:     true,
					DeactivateGlobally: true,
				},
			},
			{
				UserID:          "organizer-d",
				DisplayUsername: "coach.readonly",
				Teams:           []config.TeamRef{{ID: "team-soccer", Name: "u12-soccer"}},
				Channels:        []config.ChannelRef{{ID: "chan-chat", TeamID: "team-soccer", Name: "team-chat"}},
				Permissions: config.Permissions{
					CreateUser:         false,
					EditProfile:        false,
					ResetPassword:      false,
					ManageMembership:   true,
					RemoveFromTeam:     false,
					DeactivateGlobally: false,
				},
			},
		},
	}
}

func testLookup() *mockLookup {
	return &mockLookup{
		users: map[string]*UserInfo{
			"organizer-a": {ID: "organizer-a", Username: "coach.smith", Roles: "system_user", TeamIDs: []string{"team-soccer"}},
			"organizer-b": {ID: "organizer-b", Username: "jane.lead", Roles: "system_user", TeamIDs: []string{"team-soccer", "team-parents"}},
			"organizer-c": {ID: "organizer-c", Username: "coach.limited", Roles: "system_user", TeamIDs: []string{"team-soccer"}},
			"organizer-d": {ID: "organizer-d", Username: "coach.readonly", Roles: "system_user", TeamIDs: []string{"team-soccer"}},
			"child-1":     {ID: "child-1", Username: "child.example", Roles: "system_user", TeamIDs: []string{"team-soccer"}},
			"parent-1":    {ID: "parent-1", Username: "parent.example", Roles: "system_user", TeamIDs: []string{"team-soccer", "team-parents"}},
			"sysadmin":    {ID: "sysadmin", Username: "admin", Roles: "system_user system_admin", TeamIDs: []string{"team-soccer"}},
			"bot-1":       {ID: "bot-1", Username: "calls", Roles: "system_user", IsBot: true, TeamIDs: []string{}},
		},
	}
}

func TestAuthorizationMatrix(t *testing.T) {
	cfg := testConfig()
	lookup := testLookup()
	checker := NewChecker(cfg, lookup)

	tests := []struct {
		name      string
		actorID   string
		op        Operation
		target    Target
		wantError error
	}{
		{name: "non-organizer denied", actorID: "random-user", op: OpListUsers, wantError: ErrNotOrganizer},
		{name: "organizer list users", actorID: "organizer-a", op: OpListUsers},
		{name: "organizer list wrong team", actorID: "organizer-a", op: OpListUsers, target: Target{TeamID: "team-parents"}, wantError: ErrTeamOutOfScope},
		{name: "create user in scope", actorID: "organizer-a", op: OpCreateUser, target: Target{TeamID: "team-soccer"}},
		{name: "create user out of scope team", actorID: "organizer-a", op: OpCreateUser, target: Target{TeamID: "team-parents"}, wantError: ErrTeamOutOfScope},
		{name: "reset password scoped user", actorID: "organizer-a", op: OpResetPassword, target: Target{UserID: "child-1"}},
		{name: "reset password cross-team user", actorID: "organizer-a", op: OpResetPassword, target: Target{UserID: "parent-1"}, wantError: ErrForbidden},
		{name: "reset password peer organizer", actorID: "organizer-a", op: OpResetPassword, target: Target{UserID: "organizer-c"}, wantError: ErrProtectedTarget},
		{name: "edit profile cross-team user", actorID: "organizer-a", op: OpEditProfile, target: Target{UserID: "parent-1"}, wantError: ErrForbidden},
		{name: "edit profile peer organizer", actorID: "organizer-a", op: OpEditProfile, target: Target{UserID: "organizer-c"}, wantError: ErrProtectedTarget},
		{name: "add team member cross-team user still allowed", actorID: "organizer-a", op: OpAddTeamMember, target: Target{UserID: "parent-1", TeamID: "team-soccer"}},
		{name: "reset password sysadmin", actorID: "organizer-a", op: OpResetPassword, target: Target{UserID: "sysadmin"}, wantError: ErrProtectedTarget},
		{name: "reset password self", actorID: "organizer-a", op: OpResetPassword, target: Target{UserID: "organizer-a"}, wantError: ErrProtectedTarget},
		{name: "add team member in scope", actorID: "organizer-a", op: OpAddTeamMember, target: Target{UserID: "child-1", TeamID: "team-soccer"}},
		{name: "add team member wrong team", actorID: "organizer-a", op: OpAddTeamMember, target: Target{UserID: "child-1", TeamID: "team-parents"}, wantError: ErrTeamOutOfScope},
		{name: "remove team member in scope", actorID: "organizer-a", op: OpRemoveTeamMember, target: Target{UserID: "child-1", TeamID: "team-soccer"}},
		{name: "remove team member wrong team", actorID: "organizer-a", op: OpRemoveTeamMember, target: Target{UserID: "child-1", TeamID: "team-parents"}, wantError: ErrTeamOutOfScope},
		{name: "remove team member permission denied", actorID: "organizer-d", op: OpRemoveTeamMember, target: Target{UserID: "child-1", TeamID: "team-soccer"}, wantError: ErrPermissionDenied},
		{name: "wildcard private channel denied", actorID: "organizer-b", op: OpAddChannelMember, target: Target{UserID: "child-1", ChannelID: "chan-private", TeamID: "team-soccer", ChannelIsOpen: false}, wantError: ErrChannelOutOfScope},
		{name: "wildcard public channel allowed", actorID: "organizer-b", op: OpAddChannelMember, target: Target{UserID: "child-1", ChannelID: "chan-public", TeamID: "team-soccer", ChannelIsOpen: true}},
		{name: "explicit channel allows private", actorID: "organizer-a", op: OpAddChannelMember, target: Target{UserID: "child-1", ChannelID: "chan-chat", TeamID: "team-soccer", ChannelIsOpen: false}},
		{name: "remove channel wildcard private denied", actorID: "organizer-b", op: OpRemoveChannelMember, target: Target{UserID: "child-1", ChannelID: "chan-private", TeamID: "team-soccer", ChannelIsOpen: false}, wantError: ErrChannelOutOfScope},
		{name: "remove channel explicit allows private", actorID: "organizer-a", op: OpRemoveChannelMember, target: Target{UserID: "child-1", ChannelID: "chan-chat", TeamID: "team-soccer", ChannelIsOpen: false}},
		{name: "batch import allowed", actorID: "organizer-a", op: OpBatchImport},
		{name: "batch import permission denied", actorID: "organizer-d", op: OpBatchImport, wantError: ErrPermissionDenied},
		{name: "deactivate globally disabled", actorID: "organizer-a", op: OpDeactivateGlobal, target: Target{UserID: "child-1"}, wantError: ErrPermissionDenied},
		{name: "deactivate globally cross-team user", actorID: "organizer-c", op: OpDeactivateGlobal, target: Target{UserID: "parent-1"}, wantError: ErrForbidden},
		{name: "deactivate globally scoped only", actorID: "organizer-b", op: OpDeactivateGlobal, target: Target{UserID: "child-1"}},
		{name: "reactivate cross-team user", actorID: "organizer-c", op: OpReactivate, target: Target{UserID: "parent-1"}, wantError: ErrForbidden},
		{name: "reactivate scoped only", actorID: "organizer-b", op: OpReactivate, target: Target{UserID: "child-1"}},
		{name: "audit non-sysadmin", actorID: "organizer-a", op: OpViewAudit, wantError: ErrForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := checker.ResolveOrganizer(tt.actorID)
			if tt.wantError == ErrNotOrganizer {
				assert.ErrorIs(t, err, ErrNotOrganizer)
				return
			}
			require.NoError(t, err)

			err = checker.Authorize(ctx, tt.op, tt.target)
			if tt.wantError != nil {
				assert.ErrorIs(t, err, tt.wantError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestAuditSysadmin(t *testing.T) {
	cfg := testConfig()
	lookup := &mockLookup{
		users: map[string]*UserInfo{
			"sysadmin": {ID: "sysadmin", Username: "admin", Roles: "system_user system_admin"},
		},
	}
	checker := NewChecker(cfg, lookup)
	_, err := checker.ResolveOrganizer("sysadmin")
	require.ErrorIs(t, err, ErrNotOrganizer)

	// sysadmin not in organizer list — use direct audit check via fake organizer context won't work.
	// Audit requires actor to be system admin even if in organizer list.
	cfg.Organizers = append(cfg.Organizers, config.Organizer{
		UserID: "sysadmin",
		Teams:  []config.TeamRef{{ID: "team-soccer", Name: "u12-soccer"}},
	})
	checker = NewChecker(cfg, lookup)
	ctx, err := checker.ResolveOrganizer("sysadmin")
	require.NoError(t, err)
	assert.NoError(t, checker.Authorize(ctx, OpViewAudit, Target{}))
}

func TestIsSystemAdminExactToken(t *testing.T) {
	assert.True(t, IsSystemAdmin("system_user system_admin"))
	assert.False(t, IsSystemAdmin("system_user"))
	assert.False(t, IsSystemAdmin("not_system_admin"))
	assert.False(t, IsSystemAdmin("system_admin_extra"))
}

func TestIsProtectedSystemRoles(t *testing.T) {
	tests := []struct {
		name  string
		roles string
		want  bool
	}{
		{name: "system manager", roles: "system_manager", want: true},
		{name: "system user manager", roles: "system_user_manager", want: true},
		{name: "normal user", roles: "system_user", want: false},
		{name: "normal guest", roles: "system_guest", want: false},
		{name: "system admin", roles: "system_admin", want: true},
		{name: "normal user plus elevated role", roles: "system_user system_manager", want: true},
		{name: "normal guest plus elevated role", roles: "system_guest system_user_manager", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &UserInfo{ID: "target", Roles: tt.roles}
			assert.Equal(t, tt.want, isProtected(nil, "actor", target))
		})
	}
}

func TestUserVisibleIntersection(t *testing.T) {
	cfg := testConfig()
	checker := NewChecker(cfg, testLookup())
	ctx, err := checker.ResolveOrganizer("organizer-a")
	require.NoError(t, err)
	assert.True(t, checker.UserVisible(ctx, []string{"team-soccer", "team-parents"}))
	assert.False(t, checker.UserVisible(ctx, []string{"team-parents"}))
}

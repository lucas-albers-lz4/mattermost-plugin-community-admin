package authz

import (
	"errors"
	"testing"

	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		},
	}
}

func testLookup() *mockLookup {
	return &mockLookup{
		users: map[string]*UserInfo{
			"organizer-a": {ID: "organizer-a", Username: "coach.smith", Roles: "system_user", TeamIDs: []string{"team-soccer"}},
			"organizer-b": {ID: "organizer-b", Username: "jane.lead", Roles: "system_user", TeamIDs: []string{"team-soccer", "team-parents"}},
			"organizer-c": {ID: "organizer-c", Username: "coach.limited", Roles: "system_user", TeamIDs: []string{"team-soccer"}},
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
		{name: "reset password sysadmin", actorID: "organizer-a", op: OpResetPassword, target: Target{UserID: "sysadmin"}, wantError: ErrProtectedTarget},
		{name: "reset password self", actorID: "organizer-a", op: OpResetPassword, target: Target{UserID: "organizer-a"}, wantError: ErrProtectedTarget},
		{name: "add team member in scope", actorID: "organizer-a", op: OpAddTeamMember, target: Target{UserID: "child-1", TeamID: "team-soccer"}},
		{name: "add team member wrong team", actorID: "organizer-a", op: OpAddTeamMember, target: Target{UserID: "child-1", TeamID: "team-parents"}, wantError: ErrTeamOutOfScope},
		{name: "deactivate globally disabled", actorID: "organizer-a", op: OpDeactivateGlobal, target: Target{UserID: "child-1"}, wantError: ErrPermissionDenied},
		{name: "deactivate globally cross-team user", actorID: "organizer-c", op: OpDeactivateGlobal, target: Target{UserID: "parent-1"}, wantError: ErrForbidden},
		{name: "deactivate globally scoped only", actorID: "organizer-b", op: OpDeactivateGlobal, target: Target{UserID: "child-1"}},
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
	ctx, err := checker.ResolveOrganizer("sysadmin")
	require.ErrorIs(t, err, ErrNotOrganizer)

	// sysadmin not in organizer list — use direct audit check via fake organizer context won't work.
	// Audit requires actor to be system admin even if in organizer list.
	cfg.Organizers = append(cfg.Organizers, config.Organizer{
		UserID: "sysadmin",
		Teams:  []config.TeamRef{{ID: "team-soccer", Name: "u12-soccer"}},
	})
	checker = NewChecker(cfg, lookup)
	ctx, err = checker.ResolveOrganizer("sysadmin")
	require.NoError(t, err)
	assert.NoError(t, checker.Authorize(ctx, OpViewAudit, Target{}))
}

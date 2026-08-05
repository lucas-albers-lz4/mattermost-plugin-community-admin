package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lalbers/mattermost-plugin-community-admin/server/authz"
	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
	"github.com/lalbers/mattermost-plugin-community-admin/server/service"
)

type mapLookup map[string]*authz.UserInfo

func (m mapLookup) GetUserInfo(userID string) (*authz.UserInfo, error) {
	u, ok := m[userID]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func testScopeConfig() *config.ScopeConfig {
	return &config.ScopeConfig{
		Version:     1,
		EmailDomain: "community.local",
		SiteURL:     "https://chat.example.com",
		Organizers: []config.Organizer{
			{
				UserID:          "organizer-a",
				DisplayUsername: "coach.smith",
				Teams:           []config.TeamRef{{ID: "team-soccer", Name: "u12-soccer"}},
				Permissions: config.Permissions{
					CreateUser:       true,
					EditProfile:      true,
					ResetPassword:    true,
					ManageMembership: true,
					RemoveFromTeam:   true,
				},
				RateLimits: config.RateLimits{
					PasswordResetsPerHour: -1, // unlimited; rate limiter KV not required
				},
			},
			{
				UserID:          "organizer-c",
				DisplayUsername: "coach.limited",
				Teams:           []config.TeamRef{{ID: "team-soccer", Name: "u12-soccer"}},
				Permissions: config.Permissions{
					ResetPassword: true,
				},
			},
		},
	}
}

func testAuthPlugin(t *testing.T) *Plugin {
	t.Helper()
	p := &Plugin{
		parsedScopeConfig: testScopeConfig(),
		userLookup: mapLookup{
			"organizer-a": {ID: "organizer-a", Username: "coach.smith", Roles: "system_user", TeamIDs: []string{"team-soccer"}},
			"organizer-c": {ID: "organizer-c", Username: "coach.limited", Roles: "system_user", TeamIDs: []string{"team-soccer"}},
			"child-1":     {ID: "child-1", Username: "child.example", Roles: "system_user", TeamIDs: []string{"team-soccer"}},
		},
		rateLimitService: &service.RateLimitService{},
	}
	p.router = p.initRouter()
	return p
}

func TestAPIUnauthorized(t *testing.T) {
	p := testAuthPlugin(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()
	p.router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAPIRequireOrganizerDenied(t *testing.T) {
	p := testAuthPlugin(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Mattermost-User-Id", "stranger")
	rec := httptest.NewRecorder()
	p.router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAPIResetPasswordAuthorizeDenied(t *testing.T) {
	p := testAuthPlugin(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/organizer-c/reset-password", nil)
	req.Header.Set("Mattermost-User-Id", "organizer-a")
	rec := httptest.NewRecorder()
	p.router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "protected target", body["error"])
}

func TestAPIResetPasswordHappyPath(t *testing.T) {
	p := testAuthPlugin(t)
	p.userService = service.NewTestUserService(
		map[string]*model.User{
			"child-1": {Id: "child-1", Username: "child.example"},
		},
		func(context.Context, string, string) error { return nil },
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/child-1/reset-password", nil)
	req.Header.Set("Mattermost-User-Id", "organizer-a")
	rec := httptest.NewRecorder()
	p.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "child.example", body["username"])
	assert.NotEmpty(t, body["password"])
	assert.Contains(t, body["parent_text"], "https://chat.example.com")
}

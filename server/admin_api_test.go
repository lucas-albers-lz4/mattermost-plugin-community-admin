package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lalbers/mattermost-plugin-community-admin/server/authz"
)

func TestParsePagination(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/admin/users?page=2&per_page=25", nil)
	page, perPage := parsePagination(req, 0, 50)
	assert.Equal(t, 2, page)
	assert.Equal(t, 25, perPage)

	req = httptest.NewRequest(http.MethodGet, "/admin/users?per_page=999", nil)
	_, perPage = parsePagination(req, 0, 50)
	assert.Equal(t, 200, perPage)

	req = httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	page, perPage = parsePagination(req, 0, 50)
	assert.Equal(t, 0, page)
	assert.Equal(t, 50, perPage)
}

func TestAdminDTOs(t *testing.T) {
	t.Parallel()

	assert.Nil(t, adminUserDTO(nil))
	assert.Nil(t, adminTeamDTO(nil))
	assert.Nil(t, adminChannelDTO(nil))

	u := &model.User{
		Id:        "uid1",
		Username:  "coach.smith",
		Nickname:  "Coach",
		FirstName: "Ada",
		LastName:  "Smith",
	}
	require.Equal(t, map[string]string{
		"id":         "uid1",
		"username":   "coach.smith",
		"nickname":   "Coach",
		"first_name": "Ada",
		"last_name":  "Smith",
	}, adminUserDTO(u))

	tm := &model.Team{Id: "tid", Name: "friends-group", DisplayName: "Friends Group"}
	require.Equal(t, map[string]string{
		"id":           "tid",
		"name":         "friends-group",
		"display_name": "Friends Group",
	}, adminTeamDTO(tm))

	tmEmptyDisplay := &model.Team{Id: "tid2", Name: "slug-only"}
	require.Equal(t, "slug-only", adminTeamDTO(tmEmptyDisplay)["display_name"])

	ch := &model.Channel{Id: "cid", TeamId: "tid", Name: "town-square", DisplayName: "Town Square"}
	require.Equal(t, map[string]string{
		"id":           "cid",
		"team_id":      "tid",
		"name":         "town-square",
		"display_name": "Town Square",
	}, adminChannelDTO(ch))
}

func TestIsSystemAdminGateMatchesAuthz(t *testing.T) {
	t.Parallel()
	// Document the role string the admin list handlers rely on.
	assert.True(t, authz.IsSystemAdmin("system_user system_admin"))
	assert.False(t, authz.IsSystemAdmin("system_user"))
}

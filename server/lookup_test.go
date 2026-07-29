package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeUserOmitsEmailAndRoles(t *testing.T) {
	u := &model.User{
		Id:        "u1",
		Username:  "child.one",
		FirstName: "Child",
		LastName:  "One",
		Email:     "secret@example.com",
		Roles:     "system_user system_admin",
		DeleteAt:  0,
	}
	out := sanitizeUser(u)
	assert.Equal(t, "u1", out["id"])
	assert.Equal(t, "child.one", out["username"])
	assert.Equal(t, "Child", out["first_name"])
	assert.Equal(t, "One", out["last_name"])
	assert.Equal(t, int64(0), out["delete_at"])
	_, hasEmail := out["email"]
	_, hasRoles := out["roles"]
	assert.False(t, hasEmail)
	assert.False(t, hasRoles)
}

func TestSanitizeUserWithTeams(t *testing.T) {
	u := &model.User{Id: "u1", Username: "child.one"}

	t.Run("nil teams", func(t *testing.T) {
		out := sanitizeUserWithTeams(u, nil)
		require.Contains(t, out, "team_ids")
		assert.Nil(t, out["team_ids"])
	})

	t.Run("empty teams", func(t *testing.T) {
		out := sanitizeUserWithTeams(u, []string{})
		ids, ok := out["team_ids"].([]string)
		require.True(t, ok)
		assert.Empty(t, ids)
	})

	t.Run("populated teams", func(t *testing.T) {
		out := sanitizeUserWithTeams(u, []string{"t1", "t2"})
		ids, ok := out["team_ids"].([]string)
		require.True(t, ok)
		assert.Equal(t, []string{"t1", "t2"}, ids)
	})
}

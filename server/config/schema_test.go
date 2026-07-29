package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseScopeConfigAllFalsePermissionsPreserved(t *testing.T) {
	raw := `{
		"version": 1,
		"organizers": [{
			"user_id": "u1",
			"permissions": {
				"create_user": false,
				"edit_profile": false,
				"reset_password": false,
				"manage_membership": false,
				"remove_from_team": false,
				"deactivate_globally": false
			},
			"rate_limits": {"creates_per_hour": 5, "password_resets_per_hour": 5}
		}]
	}`
	cfg, err := ParseScopeConfig(raw)
	require.NoError(t, err)
	p := cfg.Organizers[0].Permissions
	assert.False(t, p.CreateUser)
	assert.False(t, p.EditProfile)
	assert.False(t, p.ResetPassword)
	assert.False(t, p.ManageMembership)
	assert.False(t, p.RemoveFromTeam)
	assert.False(t, p.DeactivateGlobally)
}

func TestParseScopeConfigMissingPermissionsDefaults(t *testing.T) {
	raw := `{"version":1,"organizers":[{"user_id":"u1"}]}`
	cfg, err := ParseScopeConfig(raw)
	require.NoError(t, err)
	assert.Equal(t, DefaultPermissions(), cfg.Organizers[0].Permissions)
	assert.Equal(t, DefaultRateLimits(), cfg.Organizers[0].RateLimits)
}

func TestParseScopeConfigEmptyRateLimitsObjectDefaults(t *testing.T) {
	raw := `{"version":1,"organizers":[{"user_id":"u1","rate_limits":{}}]}`
	cfg, err := ParseScopeConfig(raw)
	require.NoError(t, err)
	assert.Equal(t, DefaultRateLimits(), cfg.Organizers[0].RateLimits)
}

func TestHasChannelWildcardRequiresOpen(t *testing.T) {
	org := &Organizer{
		Channels:           []ChannelRef{{ID: "explicit-private", TeamID: "t1"}},
		AllChannelsInTeams: []string{"t1"},
	}
	assert.True(t, org.HasChannel("explicit-private", "t1", false))
	assert.True(t, org.HasChannel("any-public", "t1", true))
	assert.False(t, org.HasChannel("any-private", "t1", false))
}

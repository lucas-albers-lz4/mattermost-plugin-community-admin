package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
)

func TestApplyParsedScopeConfigKeepsPreviousOnFailure(t *testing.T) {
	goodRaw := `{"version":1,"organizers":[{"user_id":"u1","teams":[{"id":"t1","name":"soccer"}]}]}`
	good, err := applyParsedScopeConfig(nil, goodRaw)
	require.NoError(t, err)
	require.Len(t, good.Organizers, 1)
	assert.Equal(t, "u1", good.Organizers[0].UserID)

	kept, err := applyParsedScopeConfig(good, `{not-json`)
	require.Error(t, err)
	assert.Same(t, good, kept)
	assert.Equal(t, "u1", kept.Organizers[0].UserID)
}

func TestApplyParsedScopeConfigEmptyOnFirstFailure(t *testing.T) {
	empty, err := applyParsedScopeConfig(nil, `{not-json`)
	require.Error(t, err)
	require.NotNil(t, empty)
	assert.Equal(t, config.CurrentVersion, empty.Version)
	assert.Empty(t, empty.Organizers)
}

func TestApplyParsedScopeConfigReplacesOnSuccess(t *testing.T) {
	first, err := applyParsedScopeConfig(nil, `{"version":1,"organizers":[{"user_id":"u1"}]}`)
	require.NoError(t, err)
	second, err := applyParsedScopeConfig(first, `{"version":1,"organizers":[{"user_id":"u2"}]}`)
	require.NoError(t, err)
	assert.NotSame(t, first, second)
	assert.Equal(t, "u2", second.Organizers[0].UserID)
}

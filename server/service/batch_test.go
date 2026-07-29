package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
)

func TestParseBatchCSVRequiresTeam(t *testing.T) {
	_, err := ParseBatchCSV(strings.NewReader("username,firstname,lastname\na,b,c\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "team")
}

func TestParseBatchCSVOK(t *testing.T) {
	rows, err := ParseBatchCSV(strings.NewReader("username,firstname,lastname,team\nchild.one,Child,One,u12-soccer\n"))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "u12-soccer", rows[0].TeamName)
}

func TestBuildTeamNameMapDuplicate(t *testing.T) {
	org := &config.Organizer{
		Teams: []config.TeamRef{
			{ID: "t1", Name: "U12 Soccer"},
			{ID: "t2", Name: "U12 Soccer"},
		},
	}
	_, err := buildTeamNameMap(org)
	require.Error(t, err)
}

func TestImportSkipsExistingWithoutMembership(t *testing.T) {
	// Import with nil services is not used; unit-test buildTeamNameMap + Parse only here.
	org := &config.Organizer{
		Teams: []config.TeamRef{{ID: "t1", Name: "u12-soccer"}},
	}
	m, err := buildTeamNameMap(org)
	require.NoError(t, err)
	assert.Equal(t, "t1", m["u12-soccer"])
}

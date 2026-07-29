package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
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
	assert.ErrorIs(t, err, ErrBatchValidation)
}

func TestBuildTeamNameMapOK(t *testing.T) {
	org := &config.Organizer{
		Teams: []config.TeamRef{{ID: "t1", Name: "u12-soccer"}},
	}
	m, err := buildTeamNameMap(org)
	require.NoError(t, err)
	assert.Equal(t, "t1", m["u12-soccer"])
}

type stubBatchUsers struct {
	byUsername map[string]*model.User
	created    []CreateUserRequest
}

func (s *stubBatchUsers) GetByUsername(username string) (*model.User, error) {
	if u, ok := s.byUsername[username]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func (s *stubBatchUsers) CreateUser(req CreateUserRequest, _, _ string) (*CreateUserResult, error) {
	s.created = append(s.created, req)
	return &CreateUserResult{
		User:       &model.User{Id: "created-" + req.Username, Username: req.Username},
		Password:   "pw",
		ParentText: "text",
	}, nil
}

func testBatchOrg() *config.Organizer {
	return &config.Organizer{
		Teams: []config.TeamRef{{ID: "team-1", Name: "u12-soccer"}},
	}
}

func TestImportExceedsMaxRows(t *testing.T) {
	svc := &BatchImportService{users: &stubBatchUsers{}}
	rows := make([]BatchRow, MaxBatchRows+1)
	for i := range rows {
		rows[i] = BatchRow{Username: fmt.Sprintf("u%d", i), FirstName: "A", LastName: "B", TeamName: "u12-soccer"}
	}
	_, err := svc.Import(rows, testBatchOrg(), &config.ScopeConfig{}, false, nil)
	require.ErrorIs(t, err, ErrBatchValidation)
}

func TestImportSkipsExistingWithoutMembership(t *testing.T) {
	users := &stubBatchUsers{
		byUsername: map[string]*model.User{
			"child.one": {Id: "u1", Username: "child.one"},
		},
	}
	svc := &BatchImportService{users: users}
	results, err := svc.Import([]BatchRow{{
		Username: "child.one", FirstName: "Child", LastName: "One", TeamName: "u12-soccer",
	}}, testBatchOrg(), &config.ScopeConfig{}, false, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Skipped)
	assert.NotEmpty(t, results[0].SkipReason)
	assert.Empty(t, results[0].Error)
	assert.Empty(t, users.created)
}

func TestImportQuotaDenied(t *testing.T) {
	users := &stubBatchUsers{byUsername: map[string]*model.User{}}
	svc := &BatchImportService{users: users}
	quota := func() (bool, error) { return false, nil }
	results, err := svc.Import([]BatchRow{{
		Username: "child.two", FirstName: "Child", LastName: "Two", TeamName: "u12-soccer",
	}}, testBatchOrg(), &config.ScopeConfig{}, false, quota)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "rate limit exceeded", results[0].Error)
	assert.Empty(t, users.created)
}

func TestImportQuotaInfraError(t *testing.T) {
	users := &stubBatchUsers{byUsername: map[string]*model.User{}}
	svc := &BatchImportService{users: users}
	quotaErr := errors.New("kv unavailable")
	quota := func() (bool, error) { return false, quotaErr }
	_, err := svc.Import([]BatchRow{{
		Username: "child.three", FirstName: "Child", LastName: "Three", TeamName: "u12-soccer",
	}}, testBatchOrg(), &config.ScopeConfig{}, false, quota)
	require.ErrorIs(t, err, quotaErr)
	assert.False(t, errors.Is(err, ErrBatchValidation))
}

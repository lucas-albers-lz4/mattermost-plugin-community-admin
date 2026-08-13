package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubUsersAPI struct {
	byID       map[string]*model.User
	byUsername map[string]*model.User
	created    []*model.User
	createErr  error
	updateErr  error
}

func (s *stubUsersAPI) Create(user *model.User) error {
	if s.createErr != nil {
		return s.createErr
	}
	if user.Id == "" {
		user.Id = "id-" + user.Username
	}
	s.created = append(s.created, user)
	if s.byID == nil {
		s.byID = map[string]*model.User{}
	}
	s.byID[user.Id] = user
	return nil
}

func (s *stubUsersAPI) Update(user *model.User) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	return nil
}

func (s *stubUsersAPI) UpdateActive(userID string, _ bool) error {
	return nil
}

func (s *stubUsersAPI) Get(userID string) (*model.User, error) {
	if u, ok := s.byID[userID]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func (s *stubUsersAPI) GetByUsername(username string) (*model.User, error) {
	if u, ok := s.byUsername[username]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

type stubTeamsAPI struct {
	usersByTeam map[string][]*model.User
	teams       []*model.Team
	listErr     error
	memberErr   error
	members     []string
}

func (s *stubTeamsAPI) CreateMember(teamID, userID string) (*model.TeamMember, error) {
	if s.memberErr != nil {
		return nil, s.memberErr
	}
	s.members = append(s.members, teamID+":"+userID)
	return &model.TeamMember{TeamId: teamID, UserId: userID}, nil
}

func (s *stubTeamsAPI) ListUsers(teamID string, _, _ int) ([]*model.User, error) {
	return s.usersByTeam[teamID], nil
}

func (s *stubTeamsAPI) List(_ ...pluginapi.TeamListOption) ([]*model.Team, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.teams, nil
}

type stubChannelsAPI struct {
	addErr  error
	members []string
}

func (s *stubChannelsAPI) AddMember(channelID, userID string) (*model.ChannelMember, error) {
	if s.addErr != nil {
		return nil, s.addErr
	}
	s.members = append(s.members, channelID+":"+userID)
	return &model.ChannelMember{ChannelId: channelID, UserId: userID}, nil
}

type noopLog struct{}

func (noopLog) Warn(string, ...any) {}

func TestCreateUserRejectsInvalidUsername(t *testing.T) {
	svc := newUserServiceForTest(&stubUsersAPI{}, &stubTeamsAPI{}, &stubChannelsAPI{}, noopLog{})
	_, err := svc.CreateUser(CreateUserRequest{Username: "Bad Name"}, "community.local", "https://chat.example.com")
	require.Error(t, err)
}

func TestCreateUserSuccess(t *testing.T) {
	users := &stubUsersAPI{}
	teams := &stubTeamsAPI{}
	channels := &stubChannelsAPI{}
	svc := newUserServiceForTest(users, teams, channels, noopLog{})

	res, err := svc.CreateUser(CreateUserRequest{
		Username:   "child.one",
		FirstName:  "Child",
		LastName:   "One",
		Password:   "fixed-password-1!",
		TeamIDs:    []string{"t1"},
		ChannelIDs: []string{"c1"},
	}, "community.local", "https://chat.example.com")
	require.NoError(t, err)
	require.NotNil(t, res.User)
	assert.Equal(t, "child.one", res.User.Username)
	assert.Equal(t, "child.one@community.local", res.User.Email)
	assert.Equal(t, "fixed-password-1!", res.Password)
	assert.Contains(t, res.ParentText, "child.one")
	assert.Equal(t, []string{"t1:id-child.one"}, teams.members)
	assert.Equal(t, []string{"c1:id-child.one"}, channels.members)
	assert.Equal(t, "all", res.User.NotifyProps["push"])
}

func TestSearchInTeamsDedupAndFilter(t *testing.T) {
	shared := &model.User{Id: "u1", Username: "child.one", FirstName: "Child", LastName: "One"}
	other := &model.User{Id: "u2", Username: "parent.two", FirstName: "Parent", LastName: "Two"}
	teams := &stubTeamsAPI{
		usersByTeam: map[string][]*model.User{
			"t1": {shared, other},
			"t2": {shared},
		},
	}
	svc := newUserServiceForTest(&stubUsersAPI{}, teams, &stubChannelsAPI{}, noopLog{})

	all, err := svc.SearchInTeams([]string{"t1", "t2"}, "", 0, 50)
	require.NoError(t, err)
	require.Len(t, all, 2)

	filtered, err := svc.SearchInTeams([]string{"t1", "t2"}, "parent", 0, 50)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, "u2", filtered[0].Id)
}

func TestTeamIDsForUser(t *testing.T) {
	teams := &stubTeamsAPI{
		teams: []*model.Team{{Id: "t1"}, {Id: "t2"}},
	}
	svc := newUserServiceForTest(&stubUsersAPI{}, teams, &stubChannelsAPI{}, noopLog{})
	ids, err := svc.TeamIDsForUser("u1")
	require.NoError(t, err)
	assert.Equal(t, []string{"t1", "t2"}, ids)
}

func TestResetPasswordRejectsInvalidUsername(t *testing.T) {
	called := false
	svc := newUserServiceForTest(&stubUsersAPI{}, &stubTeamsAPI{}, &stubChannelsAPI{}, noopLog{})
	svc.changePassword = func(context.Context, string, string) error {
		called = true
		return nil
	}
	_, err := svc.ResetPassword("Bad User!", "https://chat.example.com")
	require.Error(t, err)
	assert.False(t, called)
}

func TestChangePasswordArgsSeparateUsernameFromFlags(t *testing.T) {
	tests := []struct {
		name     string
		username string
	}{
		{name: "leading password flag", username: "--password"},
		{name: "leading help flag", username: "-h"},
		{name: "only flags", username: "--"},
		{name: "normal username", username: "child.one"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := changePasswordArgs(tt.username, "$2b$hash")
			require.Equal(t, []string{
				"--local", "user", "change-password", "--",
				tt.username, "--password", "$2b$hash", "--hashed",
			}, args)
		})
	}
}

func TestResetPasswordSuccess(t *testing.T) {
	var gotUser, gotHash string
	svc := newUserServiceForTest(&stubUsersAPI{}, &stubTeamsAPI{}, &stubChannelsAPI{}, noopLog{})
	svc.changePassword = func(_ context.Context, username, hashed string) error {
		gotUser = username
		gotHash = hashed
		return nil
	}

	res, err := svc.ResetPassword("child.one", "https://chat.example.com")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "child.one", res.Username)
	assert.Equal(t, "child.one", gotUser)
	assert.NotEmpty(t, res.Password)
	assert.NotEqual(t, res.Password, gotHash)
	assert.True(t, strings.HasPrefix(gotHash, "$2"), "expected bcrypt hash, got %q", gotHash)
	assert.Contains(t, res.ParentText, "https://chat.example.com")
	assert.Contains(t, res.ParentText, "child.one")
	assert.Contains(t, res.ParentText, res.Password)
}

func TestResetPasswordRunnerError(t *testing.T) {
	svc := newUserServiceForTest(&stubUsersAPI{}, &stubTeamsAPI{}, &stubChannelsAPI{}, noopLog{})
	svc.changePassword = func(context.Context, string, string) error {
		return errors.New("mmctl boom")
	}
	_, err := svc.ResetPassword("child.one", "https://chat.example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mmctl boom")
}

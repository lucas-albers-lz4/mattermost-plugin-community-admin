package service

import (
	"context"
	"errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

type staticUsersAPI struct {
	byID map[string]*model.User
}

func (s *staticUsersAPI) Create(*model.User) error { return errors.New("not implemented") }

func (s *staticUsersAPI) Update(*model.User) error { return errors.New("not implemented") }

func (s *staticUsersAPI) UpdateActive(string, bool) error {
	return errors.New("not implemented")
}

func (s *staticUsersAPI) Get(userID string) (*model.User, error) {
	if u, ok := s.byID[userID]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func (s *staticUsersAPI) GetByUsername(username string) (*model.User, error) {
	for _, u := range s.byID {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

type emptyTeamsAPI struct{}

func (emptyTeamsAPI) CreateMember(string, string) (*model.TeamMember, error) {
	return nil, errors.New("not implemented")
}

func (emptyTeamsAPI) ListUsers(string, int, int) ([]*model.User, error) { return nil, nil }

func (emptyTeamsAPI) List(...pluginapi.TeamListOption) ([]*model.Team, error) {
	return nil, nil
}

type emptyChannelsAPI struct{}

func (emptyChannelsAPI) AddMember(string, string) (*model.ChannelMember, error) {
	return nil, errors.New("not implemented")
}

type discardLog struct{}

func (discardLog) Warn(string, ...any) {}

// NewTestUserService returns a UserService for handler tests with static users and an mmctl stub.
func NewTestUserService(users map[string]*model.User, changePassword func(ctx context.Context, username, hashedPassword string) error) *UserService {
	return &UserService{
		users:          &staticUsersAPI{byID: users},
		teams:          emptyTeamsAPI{},
		channels:       emptyChannelsAPI{},
		log:            discardLog{},
		changePassword: changePassword,
	}
}

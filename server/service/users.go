package service

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const mmctlPath = "/mattermost/bin/mmctl"

// UserService handles user CRUD via plugin API.
type UserService struct {
	client *pluginapi.Client
}

func NewUserService(client *pluginapi.Client) *UserService {
	return &UserService{client: client}
}

type CreateUserRequest struct {
	Username   string
	FirstName  string
	LastName   string
	Email      string
	Password   string
	TeamIDs    []string
	ChannelIDs []string
}

type CreateUserResult struct {
	User       *model.User
	Password   string
	ParentText string
}

func (s *UserService) CreateUser(req CreateUserRequest, emailDomain, siteURL string) (*CreateUserResult, error) {
	if err := ValidateUsername(req.Username); err != nil {
		return nil, err
	}

	password := req.Password
	if password == "" {
		var err error
		password, err = GeneratePassword()
		if err != nil {
			return nil, err
		}
	}

	email := req.Email
	if email == "" {
		email = req.Username + "@" + emailDomain
	}

	user := &model.User{
		Username:            strings.ToLower(req.Username),
		Email:               strings.ToLower(email),
		Password:            password,
		FirstName:           req.FirstName,
		LastName:            req.LastName,
		EmailVerified:       true,
		DisableWelcomeEmail: true,
	}

	if err := s.client.User.Create(user); err != nil {
		return nil, err
	}

	if err := ApplyPushDefaults(s.client, user); err != nil {
		return nil, err
	}

	for _, teamID := range req.TeamIDs {
		if _, err := s.client.Team.CreateMember(teamID, user.Id); err != nil {
			return nil, fmt.Errorf("add team %s: %w", teamID, err)
		}
	}

	for _, channelID := range req.ChannelIDs {
		if _, err := s.client.Channel.AddMember(channelID, user.Id); err != nil {
			return nil, fmt.Errorf("add channel %s: %w", channelID, err)
		}
	}

	return &CreateUserResult{
		User:       user,
		Password:   password,
		ParentText: ParentTextLine(siteURL, user.Username, password),
	}, nil
}

func (s *UserService) UpdateProfile(userID, firstName, lastName string) (*model.User, error) {
	user, err := s.client.User.Get(userID)
	if err != nil {
		return nil, err
	}
	user.FirstName = firstName
	user.LastName = lastName
	if err := s.client.User.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) SetActive(userID string, active bool) error {
	return s.client.User.UpdateActive(userID, active)
}

type ResetPasswordResult struct {
	Username   string
	Password   string
	ParentText string
}

// ResetPassword changes password via controlled mmctl --local exec (see docs/phase0-findings.md).
func (s *UserService) ResetPassword(username, siteURL string) (*ResetPasswordResult, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}

	password, err := GeneratePassword()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, mmctlPath, "--local", "user", "change-password", username, "--password", password) //nolint:gosec // controlled local mmctl; see SECURITY.md
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mmctl change-password: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return &ResetPasswordResult{
		Username:   username,
		Password:   password,
		ParentText: ParentTextLine(siteURL, username, password),
	}, nil
}

func (s *UserService) GetByID(userID string) (*model.User, error) {
	return s.client.User.Get(userID)
}

func (s *UserService) GetByUsername(username string) (*model.User, error) {
	return s.client.User.GetByUsername(username)
}

func (s *UserService) SearchInTeams(teamIDs []string, term string, page, perPage int) ([]*model.User, error) {
	seen := map[string]bool{}
	var results []*model.User

	for _, teamID := range teamIDs {
		users, err := s.client.Team.ListUsers(teamID, page, perPage)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			if seen[u.Id] {
				continue
			}
			if term != "" && !strings.Contains(strings.ToLower(u.Username), strings.ToLower(term)) &&
				!strings.Contains(strings.ToLower(u.FirstName+" "+u.LastName), strings.ToLower(term)) {
				continue
			}
			seen[u.Id] = true
			results = append(results, u)
		}
	}
	return results, nil
}

func (s *UserService) TeamIDsForUser(userID string) ([]string, error) {
	teams, err := s.client.Team.List(pluginapi.FilterTeamsByUser(userID))
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(teams))
	for _, t := range teams {
		ids = append(ids, t.Id)
	}
	return ids, nil
}

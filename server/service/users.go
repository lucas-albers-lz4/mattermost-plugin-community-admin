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

type userAPI interface {
	Create(user *model.User) error
	Update(user *model.User) error
	UpdateActive(userID string, active bool) error
	Get(userID string) (*model.User, error)
	GetByUsername(username string) (*model.User, error)
}

type teamUsersAPI interface {
	CreateMember(teamID, userID string) (*model.TeamMember, error)
	ListUsers(teamID string, page, perPage int) ([]*model.User, error)
	List(options ...pluginapi.TeamListOption) ([]*model.Team, error)
}

type channelUsersAPI interface {
	AddMember(channelID, userID string) (*model.ChannelMember, error)
}

type warnLogger interface {
	Warn(message string, keyValuePairs ...any)
}

// changePasswordFunc runs mmctl user change-password (hashed) for ResetPassword.
type changePasswordFunc func(ctx context.Context, username, hashedPassword string) error

// UserService handles user CRUD via plugin API.
type UserService struct {
	users          userAPI
	teams          teamUsersAPI
	channels       channelUsersAPI
	log            warnLogger
	changePassword changePasswordFunc
}

func NewUserService(client *pluginapi.Client) *UserService {
	return &UserService{
		users:    &client.User,
		teams:    &client.Team,
		channels: &client.Channel,
		log:      &client.Log,
	}
}

func newUserServiceForTest(users userAPI, teams teamUsersAPI, channels channelUsersAPI, log warnLogger) *UserService {
	return &UserService{users: users, teams: teams, channels: channels, log: log}
}

func defaultChangePassword(ctx context.Context, username, hashedPassword string) error {
	// Pass bcrypt hash via --hashed so plaintext never appears in process argv /proc/cmdline.
	cmd := exec.CommandContext(ctx, mmctlPath, changePasswordArgs(username, hashedPassword)...) //nolint:gosec // controlled local mmctl; see SECURITY.md
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mmctl change-password failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func changePasswordArgs(username, hashedPassword string) []string {
	// Flags before `--`; username after so a leading-dash name cannot be parsed as a flag,
	// and `--password` / `--hashed` remain real flags (Cobra treats post-`--` args as positionals).
	return []string{"--local", "user", "change-password", "--password", hashedPassword, "--hashed", "--", username}
}

type CreateUserRequest struct {
	Username   string
	FirstName  string
	LastName   string
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

	email := req.Username + "@" + emailDomain

	user := &model.User{
		Username:            strings.ToLower(req.Username),
		Email:               strings.ToLower(email),
		Password:            password,
		FirstName:           req.FirstName,
		LastName:            req.LastName,
		EmailVerified:       true,
		DisableWelcomeEmail: true,
	}

	if err := s.users.Create(user); err != nil {
		return nil, err
	}

	cleanup := func() {
		if err := s.users.UpdateActive(user.Id, false); err != nil && s.log != nil {
			s.log.Warn("create-user cleanup UpdateActive failed", "user_id", user.Id, "error", err.Error())
		}
	}

	if err := applyPushDefaults(s.users, user); err != nil {
		cleanup()
		return nil, err
	}

	for _, teamID := range req.TeamIDs {
		if _, err := s.teams.CreateMember(teamID, user.Id); err != nil {
			cleanup()
			return nil, fmt.Errorf("add team %s: %w", teamID, err)
		}
	}

	for _, channelID := range req.ChannelIDs {
		if _, err := s.channels.AddMember(channelID, user.Id); err != nil {
			cleanup()
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
	user, err := s.users.Get(userID)
	if err != nil {
		return nil, err
	}
	user.FirstName = firstName
	user.LastName = lastName
	if err := s.users.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) SetActive(userID string, active bool) error {
	return s.users.UpdateActive(userID, active)
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

	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	run := s.changePassword
	if run == nil {
		run = defaultChangePassword
	}
	if err := run(ctx, username, hashed); err != nil {
		return nil, err
	}

	return &ResetPasswordResult{
		Username:   username,
		Password:   password,
		ParentText: ParentTextLine(siteURL, username, password),
	}, nil
}

func (s *UserService) GetByID(userID string) (*model.User, error) {
	return s.users.Get(userID)
}

func (s *UserService) GetByUsername(username string) (*model.User, error) {
	return s.users.GetByUsername(username)
}

func (s *UserService) SearchInTeams(teamIDs []string, term string, page, perPage int) ([]*model.User, error) {
	seen := map[string]bool{}
	var results []*model.User

	for _, teamID := range teamIDs {
		users, err := s.teams.ListUsers(teamID, page, perPage)
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
	teams, err := s.teams.List(pluginapi.FilterTeamsByUser(userID))
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(teams))
	for _, t := range teams {
		ids = append(ids, t.Id)
	}
	return ids, nil
}

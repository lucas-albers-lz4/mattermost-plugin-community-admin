package command

import (
	"fmt"
	"strings"

	"github.com/lalbers/mattermost-plugin-community-admin/server/authz"
	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
	"github.com/lalbers/mattermost-plugin-community-admin/server/service"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const commandTrigger = "community-admin"

type ScopeConfigLoader func() *config.ScopeConfig

type Command interface {
	Handle(args *model.CommandArgs) (*model.CommandResponse, error)
	SetScopeConfigLoader(loader ScopeConfigLoader)
}

type Handler struct {
	client            *pluginapi.Client
	scopeConfigLoader ScopeConfigLoader
	userService       *service.UserService
	membershipService *service.MembershipService
}

func NewCommandHandler(client *pluginapi.Client, loader ScopeConfigLoader) Command {
	err := client.SlashCommand.Register(&model.Command{
		Trigger:          commandTrigger,
		AutoComplete:     true,
		AutoCompleteDesc: "Community user management (organizers)",
		AutoCompleteHint: "[reset-password USERNAME | remove-from-team USERNAME TEAM]",
		AutocompleteData: model.NewAutocompleteData(commandTrigger, "[action]", "Organizer actions for mobile"),
	})
	if err != nil {
		client.Log.Error("Failed to register slash command", "error", err)
	}
	return &Handler{
		client:            client,
		scopeConfigLoader: loader,
		userService:       service.NewUserService(client),
		membershipService: service.NewMembershipService(client),
	}
}

func (h *Handler) SetScopeConfigLoader(loader ScopeConfigLoader) {
	h.scopeConfigLoader = loader
}

func (h *Handler) Handle(args *model.CommandArgs) (*model.CommandResponse, error) {
	cfg := h.scopeConfigLoader()
	checker := authz.NewChecker(cfg, &pluginLookup{client: h.client})
	orgCtx, err := checker.ResolveOrganizer(args.UserId)
	if err != nil {
		return ephemeral("Community Admin: you are not an organizer."), nil
	}

	fields := strings.Fields(args.Command)
	if len(fields) < 2 {
		return ephemeral("Usage: /community-admin reset-password USERNAME | remove-from-team USERNAME TEAM_NAME"), nil
	}

	action := strings.ToLower(fields[1])
	switch action {
	case "reset-password":
		if len(fields) < 3 {
			return ephemeral("Usage: /community-admin reset-password USERNAME"), nil
		}
		return h.resetPassword(orgCtx, checker, args, fields[2], cfg.SiteURL)
	case "remove-from-team":
		if len(fields) < 4 {
			return ephemeral("Usage: /community-admin remove-from-team USERNAME TEAM_NAME"), nil
		}
		return h.removeFromTeam(orgCtx, checker, args, fields[2], fields[3])
	default:
		return ephemeral(fmt.Sprintf("Unknown action: %s", action)), nil
	}
}

func (h *Handler) resetPassword(orgCtx *authz.OrganizerContext, checker *authz.Checker, args *model.CommandArgs, username, siteURL string) (*model.CommandResponse, error) {
	target, err := h.client.User.GetByUsername(username)
	if err != nil {
		return ephemeral("User not found."), nil
	}
	if err := checker.Authorize(orgCtx, authz.OpResetPassword, authz.Target{UserID: target.Id}); err != nil {
		return ephemeral("Not allowed to reset password for that user."), nil
	}
	result, err := h.userService.ResetPassword(username, siteURL)
	if err != nil {
		return ephemeral("Password reset failed."), nil
	}
	return ephemeral(fmt.Sprintf("Password reset for **%s**.\n\n%s", username, result.ParentText)), nil
}

func (h *Handler) removeFromTeam(orgCtx *authz.OrganizerContext, checker *authz.Checker, args *model.CommandArgs, username, teamName string) (*model.CommandResponse, error) {
	target, err := h.client.User.GetByUsername(username)
	if err != nil {
		return ephemeral("User not found."), nil
	}
	team, err := h.client.Team.GetByName(teamName)
	if err != nil {
		return ephemeral("Team not found."), nil
	}
	if err := checker.Authorize(orgCtx, authz.OpRemoveTeamMember, authz.Target{UserID: target.Id, TeamID: team.Id}); err != nil {
		return ephemeral("Not allowed to remove that user from the team."), nil
	}
	if err := h.membershipService.RemoveTeamMember(team.Id, target.Id, args.UserId); err != nil {
		return ephemeral("Failed to remove user from team."), nil
	}
	return ephemeral(fmt.Sprintf("Removed **%s** from team **%s**.", username, teamName)), nil
}

func ephemeral(text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
	}
}

type pluginLookup struct {
	client *pluginapi.Client
}

func (l *pluginLookup) GetUserInfo(userID string) (*authz.UserInfo, error) {
	user, err := l.client.User.Get(userID)
	if err != nil {
		return nil, err
	}
	teams, err := l.client.Team.List(pluginapi.FilterTeamsByUser(userID))
	if err != nil {
		return nil, err
	}
	teamIDs := make([]string, 0, len(teams))
	for _, t := range teams {
		teamIDs = append(teamIDs, t.Id)
	}
	return &authz.UserInfo{
		ID: user.Id, Username: user.Username, Roles: user.Roles, IsBot: user.IsBot, TeamIDs: teamIDs,
	}, nil
}

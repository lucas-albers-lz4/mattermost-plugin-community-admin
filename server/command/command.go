package command

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/pkg/errors"

	"github.com/lalbers/mattermost-plugin-community-admin/server/authz"
	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
	"github.com/lalbers/mattermost-plugin-community-admin/server/service"
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
	auditService      *service.AuditService
	rateLimitService  *service.RateLimitService
}

type Dependencies struct {
	UserService       *service.UserService
	MembershipService *service.MembershipService
	AuditService      *service.AuditService
	RateLimitService  *service.RateLimitService
}

func NewCommandHandler(client *pluginapi.Client, loader ScopeConfigLoader, deps Dependencies) (Command, error) {
	err := client.SlashCommand.Register(&model.Command{
		Trigger:          commandTrigger,
		AutoComplete:     true,
		AutoCompleteDesc: "Community user management (organizers)",
		AutoCompleteHint: "[reset-password USERNAME | remove-from-team USERNAME TEAM]",
		AutocompleteData: model.NewAutocompleteData(commandTrigger, "[action]", "Organizer actions for mobile"),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to register slash command")
	}
	return &Handler{
		client:            client,
		scopeConfigLoader: loader,
		userService:       deps.UserService,
		membershipService: deps.MembershipService,
		auditService:      deps.AuditService,
		rateLimitService:  deps.RateLimitService,
	}, nil
}

func (h *Handler) SetScopeConfigLoader(loader ScopeConfigLoader) {
	h.scopeConfigLoader = loader
}

func (h *Handler) Handle(args *model.CommandArgs) (*model.CommandResponse, error) {
	cfg := h.scopeConfigLoader()
	checker := authz.NewChecker(cfg, authz.NewPluginUserLookup(h.client))
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
		return h.resetPassword(orgCtx, checker, args, fields[2], cfg)
	case "remove-from-team":
		if len(fields) < 4 {
			return ephemeral("Usage: /community-admin remove-from-team USERNAME TEAM_NAME"), nil
		}
		teamName := strings.Join(fields[3:], " ")
		return h.removeFromTeam(orgCtx, checker, args, fields[2], teamName)
	default:
		return ephemeral(fmt.Sprintf("Unknown action: %s", action)), nil
	}
}

func (h *Handler) resetPassword(orgCtx *authz.OrganizerContext, checker *authz.Checker, args *model.CommandArgs, username string, cfg *config.ScopeConfig) (*model.CommandResponse, error) {
	target, err := h.client.User.GetByUsername(username)
	if err != nil {
		return ephemeral("User not found."), nil
	}
	if err := checker.Authorize(orgCtx, authz.OpResetPassword, authz.Target{UserID: target.Id}); err != nil {
		return ephemeral("Not allowed to reset password for that user."), nil
	}

	ok, err := h.rateLimitService.CheckAndIncrement(args.UserId, "reset_password", orgCtx.Organizer.RateLimits.EffectivePasswordResetsPerHour())
	if err != nil {
		return ephemeral("Rate limit check failed."), nil
	}
	if !ok {
		return ephemeral("Rate limit exceeded for password resets."), nil
	}

	result, err := h.userService.ResetPassword(username, cfg.SiteURL)
	if err != nil {
		return ephemeral("Password reset failed."), nil
	}

	if err := h.auditService.Record(service.AuditEntry{
		ActorID:        args.UserId,
		ActorUsername:  actorUsername(h.client, args.UserId),
		Action:         "reset_password",
		TargetID:       target.Id,
		TargetUsername: target.Username,
	}); err != nil {
		h.client.Log.Warn("audit record failed", "action", "reset_password", "error", err.Error())
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

	if err := h.auditService.Record(service.AuditEntry{
		ActorID:        args.UserId,
		ActorUsername:  actorUsername(h.client, args.UserId),
		Action:         "remove_team_member",
		TargetID:       target.Id,
		TargetUsername: target.Username,
		TeamID:         team.Id,
	}); err != nil {
		h.client.Log.Warn("audit record failed", "action", "remove_team_member", "error", err.Error())
	}

	return ephemeral(fmt.Sprintf("Removed **%s** from team **%s**.", username, teamName)), nil
}

func ephemeral(text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
	}
}

func actorUsername(client *pluginapi.Client, actorID string) string {
	user, err := client.User.Get(actorID)
	if err != nil {
		return ""
	}
	return user.Username
}

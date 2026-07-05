package authz

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
)

var (
	ErrNotOrganizer      = errors.New("not an organizer")
	ErrForbidden         = errors.New("forbidden")
	ErrProtectedTarget   = errors.New("protected target")
	ErrTeamOutOfScope    = errors.New("team out of scope")
	ErrChannelOutOfScope = errors.New("channel out of scope")
	ErrUserOutOfScope    = errors.New("user out of scope")
	ErrPermissionDenied  = errors.New("permission denied")
)

// Checker enforces organizer scope for all operations.
type Checker struct {
	cfg    *config.ScopeConfig
	lookup UserLookup
}

func NewChecker(cfg *config.ScopeConfig, lookup UserLookup) *Checker {
	return &Checker{cfg: cfg, lookup: lookup}
}

// ResolveOrganizer returns organizer context or ErrNotOrganizer.
func (c *Checker) ResolveOrganizer(actorID string) (*OrganizerContext, error) {
	if actorID == "" {
		return nil, ErrNotOrganizer
	}
	org := c.cfg.FindOrganizer(actorID)
	if org == nil {
		return nil, ErrNotOrganizer
	}
	return &OrganizerContext{ActorID: actorID, Organizer: org, Config: c.cfg}, nil
}

// IsSystemAdmin returns true if actor has manage_system via roles string.
func IsSystemAdmin(roles string) bool {
	return strings.Contains(roles, "system_admin")
}

func isProtected(actorID string, target *UserInfo) bool {
	if target == nil {
		return false
	}
	if target.ID == actorID {
		return true
	}
	if target.IsBot {
		return true
	}
	if IsSystemAdmin(target.Roles) {
		return true
	}
	if target.Username == "calls" {
		return true
	}
	return false
}

func userInOrganizerTeams(org *config.Organizer, teamIDs []string) bool {
	return slices.ContainsFunc(teamIDs, org.HasTeam)
}

func allTeamsSubset(org *config.Organizer, userTeamIDs []string) bool {
	if len(userTeamIDs) == 0 {
		return true
	}
	for _, tid := range userTeamIDs {
		if !org.HasTeam(tid) {
			return false
		}
	}
	return true
}

// Authorize checks whether actor may perform op on target.
func (c *Checker) Authorize(ctx *OrganizerContext, op Operation, target Target) error {
	if ctx == nil {
		return ErrNotOrganizer
	}

	if op == OpViewAudit {
		actor, err := c.lookup.GetUserInfo(ctx.ActorID)
		if err != nil {
			return ErrForbidden
		}
		if !IsSystemAdmin(actor.Roles) {
			return ErrForbidden
		}
		return nil
	}

	if ctx.Organizer == nil {
		return ErrNotOrganizer
	}
	org := ctx.Organizer

	switch op {
	case OpListUsers:
		if target.TeamID != "" && !org.HasTeam(target.TeamID) {
			return ErrTeamOutOfScope
		}
		return nil

	case OpCreateUser:
		if !org.Permissions.CreateUser {
			return ErrPermissionDenied
		}
		if target.TeamID != "" && !org.HasTeam(target.TeamID) {
			return ErrTeamOutOfScope
		}
		for _, chTeam := range []string{target.TeamID} {
			if target.ChannelID != "" && chTeam != "" && !org.HasChannel(target.ChannelID, chTeam) {
				return ErrChannelOutOfScope
			}
		}
		return nil

	case OpBatchImport:
		if !org.Permissions.CreateUser {
			return ErrPermissionDenied
		}
		return nil

	case OpEditProfile:
		if !org.Permissions.EditProfile {
			return ErrPermissionDenied
		}
		return c.authorizeTargetUser(ctx, target.UserID)

	case OpResetPassword:
		if !org.Permissions.ResetPassword {
			return ErrPermissionDenied
		}
		return c.authorizeTargetUser(ctx, target.UserID)

	case OpAddTeamMember, OpRemoveTeamMember:
		if op == OpRemoveTeamMember {
			if !org.Permissions.RemoveFromTeam {
				return ErrPermissionDenied
			}
		} else if !org.Permissions.ManageMembership {
			return ErrPermissionDenied
		}
		if !org.HasTeam(target.TeamID) {
			return ErrTeamOutOfScope
		}
		if target.UserID != "" {
			return c.authorizeTargetUser(ctx, target.UserID)
		}
		return nil

	case OpAddChannelMember, OpRemoveChannelMember:
		if !org.Permissions.ManageMembership {
			return ErrPermissionDenied
		}
		if !org.HasChannel(target.ChannelID, target.TeamID) {
			return ErrChannelOutOfScope
		}
		if target.UserID != "" {
			return c.authorizeTargetUser(ctx, target.UserID)
		}
		return nil

	case OpDeactivateGlobal, OpReactivate:
		if !org.Permissions.DeactivateGlobally {
			return ErrPermissionDenied
		}
		targetUser, err := c.lookup.GetUserInfo(target.UserID)
		if err != nil {
			return ErrUserOutOfScope
		}
		if isProtected(ctx.ActorID, targetUser) {
			return ErrProtectedTarget
		}
		if !userInOrganizerTeams(org, targetUser.TeamIDs) {
			return ErrUserOutOfScope
		}
		if !allTeamsSubset(org, targetUser.TeamIDs) {
			return fmt.Errorf("%w: user belongs to teams outside organizer scope", ErrForbidden)
		}
		return nil

	default:
		return ErrForbidden
	}
}

func (c *Checker) authorizeTargetUser(ctx *OrganizerContext, targetUserID string) error {
	if targetUserID == "" {
		return ErrUserOutOfScope
	}
	targetUser, err := c.lookup.GetUserInfo(targetUserID)
	if err != nil {
		return ErrUserOutOfScope
	}
	if isProtected(ctx.ActorID, targetUser) {
		return ErrProtectedTarget
	}
	if !userInOrganizerTeams(ctx.Organizer, targetUser.TeamIDs) {
		return ErrUserOutOfScope
	}
	return nil
}

// UserVisible returns true if user is visible in organizer scope (member of scoped team).
func (c *Checker) UserVisible(ctx *OrganizerContext, userTeamIDs []string) bool {
	return userInOrganizerTeams(ctx.Organizer, userTeamIDs)
}

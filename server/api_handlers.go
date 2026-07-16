package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"

	"github.com/lalbers/mattermost-plugin-community-admin/server/authz"
	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
	"github.com/lalbers/mattermost-plugin-community-admin/server/service"
)

func (p *Plugin) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		p.API.LogError("failed to encode JSON", "error", err.Error())
	}
}

func (p *Plugin) writeError(w http.ResponseWriter, err error, fallbackStatus int) {
	switch {
	case errors.Is(err, authz.ErrNotOrganizer):
		p.writeJSON(w, http.StatusForbidden, map[string]string{"error": "not an organizer"})
	case errors.Is(err, authz.ErrForbidden):
		p.writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, authz.ErrProtectedTarget):
		p.writeJSON(w, http.StatusForbidden, map[string]string{"error": "protected target"})
	case errors.Is(err, authz.ErrTeamOutOfScope), errors.Is(err, authz.ErrChannelOutOfScope), errors.Is(err, authz.ErrUserOutOfScope):
		p.writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case errors.Is(err, authz.ErrPermissionDenied):
		p.writeJSON(w, http.StatusForbidden, map[string]string{"error": "permission denied"})
	default:
		if fallbackStatus == 0 {
			fallbackStatus = http.StatusInternalServerError
		}
		p.writeJSON(w, fallbackStatus, map[string]string{"error": err.Error()})
	}
}

func (p *Plugin) actorID(r *http.Request) string {
	return r.Header.Get("Mattermost-User-Id")
}

func (p *Plugin) organizerContext(actorID string) (*authz.OrganizerContext, error) {
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	return checker.ResolveOrganizer(actorID)
}

func (p *Plugin) handleMe(w http.ResponseWriter, r *http.Request) {
	actorID := p.actorID(r)
	ctx, err := p.organizerContext(actorID)
	if err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	teams, channels := p.scopeChoicesForOrganizer(ctx.Organizer)
	p.writeJSON(w, http.StatusOK, map[string]any{
		"user_id":          ctx.ActorID,
		"display_username": ctx.Organizer.DisplayUsername,
		"teams":            teams,
		"channels":         channels,
		"permissions":      ctx.Organizer.Permissions,
		"site_url":         ctx.Config.SiteURL,
	})
}

// scopeChoicesForOrganizer returns team/channel options for the Create User UI.
// Explicit Channels are included; for each team in AllChannelsInTeams, public
// channels are listed live so dropdowns are not empty when ScopeEditor defaults
// to all_channels_in_teams with channels:[].
func (p *Plugin) scopeChoicesForOrganizer(org *config.Organizer) ([]config.TeamRef, []config.ChannelRef) {
	teams := make([]config.TeamRef, 0, len(org.Teams))
	for _, t := range org.Teams {
		if t.ID == "" {
			continue
		}
		ref := t
		if tm, err := p.client.Team.Get(t.ID); err == nil && tm != nil {
			if tm.DisplayName != "" {
				ref.Name = tm.DisplayName
			} else if tm.Name != "" {
				ref.Name = tm.Name
			}
		}
		if ref.Name == "" {
			ref.Name = ref.ID
		}
		teams = append(teams, ref)
	}

	seen := make(map[string]struct{}, len(org.Channels))
	channels := make([]config.ChannelRef, 0, len(org.Channels))
	for _, ch := range org.Channels {
		if ch.ID == "" {
			continue
		}
		ref := ch
		if c, err := p.client.Channel.Get(ch.ID); err == nil && c != nil {
			if c.DisplayName != "" {
				ref.Name = c.DisplayName
			} else if c.Name != "" {
				ref.Name = c.Name
			}
			if ref.TeamID == "" {
				ref.TeamID = c.TeamId
			}
		}
		if ref.Name == "" {
			ref.Name = ref.ID
		}
		seen[ref.ID] = struct{}{}
		channels = append(channels, ref)
	}

	for _, teamID := range org.AllChannelsInTeams {
		if teamID == "" {
			continue
		}
		page := 0
		for {
			list, err := p.client.Channel.ListPublicChannelsForTeam(teamID, page, 200)
			if err != nil {
				p.API.LogWarn("failed to list public channels for scope UI", "team_id", teamID, "error", err.Error())
				break
			}
			if len(list) == 0 {
				break
			}
			for _, ch := range list {
				if ch == nil || ch.Id == "" {
					continue
				}
				if _, ok := seen[ch.Id]; ok {
					continue
				}
				seen[ch.Id] = struct{}{}
				name := ch.DisplayName
				if name == "" {
					name = ch.Name
				}
				channels = append(channels, config.ChannelRef{
					ID:     ch.Id,
					TeamID: ch.TeamId,
					Name:   name,
				})
			}
			if len(list) < 200 {
				break
			}
			page++
		}
	}

	return teams, channels
}

func (p *Plugin) handleListUsers(w http.ResponseWriter, r *http.Request) {
	actorID := p.actorID(r)
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	orgCtx, err := checker.ResolveOrganizer(actorID)
	if err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}

	teamID := r.URL.Query().Get("team_id")
	term := r.URL.Query().Get("q")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage <= 0 {
		perPage = 50
	}

	target := authz.Target{TeamID: teamID}
	if err := checker.Authorize(orgCtx, authz.OpListUsers, target); err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}

	teamIDs := orgCtx.Organizer.TeamIDs()
	if teamID != "" {
		teamIDs = []string{teamID}
	}

	users, err := p.userService.SearchInTeams(teamIDs, term, page, perPage)
	if err != nil {
		p.writeError(w, err, http.StatusInternalServerError)
		return
	}

	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		teamIDsForUser, _ := p.userService.TeamIDsForUser(u.Id)
		if !checker.UserVisible(orgCtx, teamIDsForUser) {
			continue
		}
		out = append(out, sanitizeUser(u))
	}
	p.writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

type createUserBody struct {
	Username   string   `json:"username"`
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	TeamIDs    []string `json:"team_ids"`
	ChannelIDs []string `json:"channel_ids"`
}

func (p *Plugin) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	actorID := p.actorID(r)
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	orgCtx, err := checker.ResolveOrganizer(actorID)
	if err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}

	var body createUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		p.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if len(body.TeamIDs) == 0 {
		p.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one team_id is required"})
		return
	}

	teamID := ""
	if len(body.TeamIDs) > 0 {
		teamID = body.TeamIDs[0]
	}
	if err := checker.Authorize(orgCtx, authz.OpCreateUser, authz.Target{TeamID: teamID}); err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}

	for _, tid := range body.TeamIDs {
		if !orgCtx.Organizer.HasTeam(tid) {
			p.writeError(w, authz.ErrTeamOutOfScope, http.StatusForbidden)
			return
		}
	}
	for _, cid := range body.ChannelIDs {
		chTeam, err := p.membershipService.GetChannelTeamID(cid)
		if err != nil || !orgCtx.Organizer.HasChannel(cid, chTeam) {
			p.writeError(w, authz.ErrChannelOutOfScope, http.StatusForbidden)
			return
		}
	}

	ok, err := p.rateLimitService.CheckAndIncrement(actorID, "create_user", orgCtx.Organizer.RateLimits.CreatesPerHour)
	if err != nil {
		p.writeError(w, err, http.StatusInternalServerError)
		return
	}
	if !ok {
		p.writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}

	result, err := p.userService.CreateUser(service.CreateUserRequest{
		Username:   body.Username,
		FirstName:  body.FirstName,
		LastName:   body.LastName,
		TeamIDs:    body.TeamIDs,
		ChannelIDs: body.ChannelIDs,
	}, cfg.EmailDomain, cfg.SiteURL)
	if err != nil {
		p.writeError(w, err, http.StatusBadRequest)
		return
	}

	_ = p.auditService.Record(service.AuditEntry{
		ActorID:        actorID,
		ActorUsername:  actorUsername(p.client, actorID),
		Action:         "create_user",
		TargetID:       result.User.Id,
		TargetUsername: result.User.Username,
		ClientIP:       pluginContextIP(r),
	})

	p.writeJSON(w, http.StatusCreated, map[string]any{
		"user":        sanitizeUser(result.User),
		"password":    result.Password,
		"parent_text": result.ParentText,
	})
}

func (p *Plugin) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	actorID := p.actorID(r)
	userID := mux.Vars(r)["id"]
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	orgCtx, err := checker.ResolveOrganizer(actorID)
	if err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := checker.Authorize(orgCtx, authz.OpEditProfile, authz.Target{UserID: userID}); err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}

	var body struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		p.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	user, err := p.userService.UpdateProfile(userID, body.FirstName, body.LastName)
	if err != nil {
		p.writeError(w, err, http.StatusBadRequest)
		return
	}

	_ = p.auditService.Record(service.AuditEntry{
		ActorID: actorID, ActorUsername: actorUsername(p.client, actorID),
		Action: "edit_profile", TargetID: userID, TargetUsername: user.Username,
		ClientIP: pluginContextIP(r),
	})
	p.writeJSON(w, http.StatusOK, sanitizeUser(user))
}

func (p *Plugin) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	actorID := p.actorID(r)
	userID := mux.Vars(r)["id"]
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	orgCtx, err := checker.ResolveOrganizer(actorID)
	if err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := checker.Authorize(orgCtx, authz.OpResetPassword, authz.Target{UserID: userID}); err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}

	ok, err := p.rateLimitService.CheckAndIncrement(actorID, "reset_password", orgCtx.Organizer.RateLimits.PasswordResetsPerHour)
	if err != nil {
		p.writeError(w, err, http.StatusInternalServerError)
		return
	}
	if !ok {
		p.writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}

	target, err := p.userService.GetByID(userID)
	if err != nil {
		p.writeError(w, err, http.StatusNotFound)
		return
	}

	result, err := p.userService.ResetPassword(target.Username, cfg.SiteURL)
	if err != nil {
		p.writeError(w, err, http.StatusInternalServerError)
		return
	}

	_ = p.auditService.Record(service.AuditEntry{
		ActorID: actorID, ActorUsername: actorUsername(p.client, actorID),
		Action: "reset_password", TargetID: userID, TargetUsername: target.Username,
		ClientIP: pluginContextIP(r),
	})

	p.writeJSON(w, http.StatusOK, map[string]any{
		"username":    result.Username,
		"password":    result.Password,
		"parent_text": result.ParentText,
	})
}

func (p *Plugin) handleActivate(w http.ResponseWriter, r *http.Request) {
	p.handleSetActive(w, r, true)
}

func (p *Plugin) handleDeactivate(w http.ResponseWriter, r *http.Request) {
	p.handleSetActive(w, r, false)
}

func (p *Plugin) handleSetActive(w http.ResponseWriter, r *http.Request, active bool) {
	actorID := p.actorID(r)
	userID := mux.Vars(r)["id"]
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	orgCtx, err := checker.ResolveOrganizer(actorID)
	if err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	op := authz.OpDeactivateGlobal
	if active {
		op = authz.OpReactivate
	}
	if err := checker.Authorize(orgCtx, op, authz.Target{UserID: userID}); err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := p.userService.SetActive(userID, active); err != nil {
		p.writeError(w, err, http.StatusBadRequest)
		return
	}
	action := "deactivate_global"
	if active {
		action = "reactivate"
	}
	target, _ := p.userService.GetByID(userID)
	username := ""
	if target != nil {
		username = target.Username
	}
	_ = p.auditService.Record(service.AuditEntry{
		ActorID: actorID, ActorUsername: actorUsername(p.client, actorID),
		Action: action, TargetID: userID, TargetUsername: username,
		ClientIP: pluginContextIP(r),
	})
	p.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (p *Plugin) handleAddTeamMember(w http.ResponseWriter, r *http.Request) {
	actorID := p.actorID(r)
	userID := mux.Vars(r)["id"]
	teamID := mux.Vars(r)["teamId"]
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	orgCtx, err := checker.ResolveOrganizer(actorID)
	if err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := checker.Authorize(orgCtx, authz.OpAddTeamMember, authz.Target{UserID: userID, TeamID: teamID}); err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := p.membershipService.AddTeamMember(teamID, userID); err != nil {
		p.writeError(w, err, http.StatusBadRequest)
		return
	}
	_ = p.auditService.Record(service.AuditEntry{
		ActorID: actorID, ActorUsername: actorUsername(p.client, actorID),
		Action: "add_team_member", TargetID: userID, TeamID: teamID,
		ClientIP: pluginContextIP(r),
	})
	p.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (p *Plugin) handleRemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	actorID := p.actorID(r)
	userID := mux.Vars(r)["id"]
	teamID := mux.Vars(r)["teamId"]
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	orgCtx, err := checker.ResolveOrganizer(actorID)
	if err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := checker.Authorize(orgCtx, authz.OpRemoveTeamMember, authz.Target{UserID: userID, TeamID: teamID}); err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := p.membershipService.RemoveTeamMember(teamID, userID, actorID); err != nil {
		p.writeError(w, err, http.StatusBadRequest)
		return
	}
	_ = p.auditService.Record(service.AuditEntry{
		ActorID: actorID, ActorUsername: actorUsername(p.client, actorID),
		Action: "remove_team_member", TargetID: userID, TeamID: teamID,
		ClientIP: pluginContextIP(r),
	})
	p.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (p *Plugin) handleAddChannelMember(w http.ResponseWriter, r *http.Request) {
	actorID := p.actorID(r)
	userID := mux.Vars(r)["id"]
	channelID := mux.Vars(r)["channelId"]
	chTeam, err := p.membershipService.GetChannelTeamID(channelID)
	if err != nil {
		p.writeError(w, err, http.StatusBadRequest)
		return
	}
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	orgCtx, err := checker.ResolveOrganizer(actorID)
	if err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := checker.Authorize(orgCtx, authz.OpAddChannelMember, authz.Target{UserID: userID, ChannelID: channelID, TeamID: chTeam}); err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := p.membershipService.AddChannelMember(channelID, userID); err != nil {
		p.writeError(w, err, http.StatusBadRequest)
		return
	}
	_ = p.auditService.Record(service.AuditEntry{
		ActorID: actorID, ActorUsername: actorUsername(p.client, actorID),
		Action: "add_channel_member", TargetID: userID, ChannelID: channelID, TeamID: chTeam,
		ClientIP: pluginContextIP(r),
	})
	p.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (p *Plugin) handleRemoveChannelMember(w http.ResponseWriter, r *http.Request) {
	actorID := p.actorID(r)
	userID := mux.Vars(r)["id"]
	channelID := mux.Vars(r)["channelId"]
	chTeam, err := p.membershipService.GetChannelTeamID(channelID)
	if err != nil {
		p.writeError(w, err, http.StatusBadRequest)
		return
	}
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	orgCtx, err := checker.ResolveOrganizer(actorID)
	if err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := checker.Authorize(orgCtx, authz.OpRemoveChannelMember, authz.Target{UserID: userID, ChannelID: channelID, TeamID: chTeam}); err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := p.membershipService.RemoveChannelMember(channelID, userID); err != nil {
		p.writeError(w, err, http.StatusBadRequest)
		return
	}
	_ = p.auditService.Record(service.AuditEntry{
		ActorID: actorID, ActorUsername: actorUsername(p.client, actorID),
		Action: "remove_channel_member", TargetID: userID, ChannelID: channelID, TeamID: chTeam,
		ClientIP: pluginContextIP(r),
	})
	p.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (p *Plugin) handleAudit(w http.ResponseWriter, r *http.Request) {
	actorID := p.actorID(r)
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	orgCtx := &authz.OrganizerContext{ActorID: actorID, Config: cfg}
	if err := checker.Authorize(orgCtx, authz.OpViewAudit, authz.Target{}); err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := p.auditService.List(limit)
	if err != nil {
		p.writeError(w, err, http.StatusInternalServerError)
		return
	}
	p.writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (p *Plugin) handleBatchImport(w http.ResponseWriter, r *http.Request) {
	actorID := p.actorID(r)
	cfg := p.getScopeConfig()
	checker := authz.NewChecker(cfg, newPluginUserLookup(p.client))
	orgCtx, err := checker.ResolveOrganizer(actorID)
	if err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}
	if err := checker.Authorize(orgCtx, authz.OpBatchImport, authz.Target{}); err != nil {
		p.writeError(w, err, http.StatusForbidden)
		return
	}

	dryRun := r.URL.Query().Get("dry_run") == "true"
	rows, err := service.ParseBatchCSV(r.Body)
	if err != nil {
		p.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	results, err := p.batchService.Import(rows, orgCtx.Organizer, cfg, dryRun)
	if err != nil {
		p.writeError(w, err, http.StatusInternalServerError)
		return
	}
	for _, res := range results {
		if res.Created {
			_ = p.auditService.Record(service.AuditEntry{
				ActorID: actorID, ActorUsername: actorUsername(p.client, actorID),
				Action: "batch_create_user", TargetUsername: res.Username,
				ClientIP: pluginContextIP(r),
			})
		}
	}
	p.writeJSON(w, http.StatusOK, map[string]any{"results": results, "dry_run": dryRun})
}

// requireSystemAdmin writes 403 and returns false unless the caller is a system admin.
func (p *Plugin) requireSystemAdmin(w http.ResponseWriter, r *http.Request) bool {
	actorID := p.actorID(r)
	user, err := p.client.User.Get(actorID)
	if err != nil || user == nil || !authz.IsSystemAdmin(user.Roles) {
		p.writeJSON(w, http.StatusForbidden, map[string]string{"error": "system admin required"})
		return false
	}
	return true
}

func parsePagination(r *http.Request, defaultPage, defaultPerPage int) (page, perPage int) {
	page = defaultPage
	perPage = defaultPerPage
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			perPage = n
		}
	}
	if perPage > 200 {
		perPage = 200
	}
	return page, perPage
}

func adminUserDTO(u *model.User) map[string]string {
	if u == nil {
		return nil
	}
	return map[string]string{
		"id":         u.Id,
		"username":   u.Username,
		"nickname":   u.Nickname,
		"first_name": u.FirstName,
		"last_name":  u.LastName,
	}
}

func adminTeamDTO(t *model.Team) map[string]string {
	if t == nil {
		return nil
	}
	display := t.DisplayName
	if display == "" {
		display = t.Name
	}
	return map[string]string{
		"id":           t.Id,
		"name":         t.Name,
		"display_name": display,
	}
}

func adminChannelDTO(ch *model.Channel) map[string]string {
	if ch == nil {
		return nil
	}
	display := ch.DisplayName
	if display == "" {
		display = ch.Name
	}
	return map[string]string{
		"id":           ch.Id,
		"team_id":      ch.TeamId,
		"name":         ch.Name,
		"display_name": display,
	}
}

func (p *Plugin) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	if !p.requireSystemAdmin(w, r) {
		return
	}
	page, perPage := parsePagination(r, 0, 50)
	term := strings.TrimSpace(r.URL.Query().Get("term"))

	var users []*model.User
	var err error
	if term != "" {
		users, err = p.client.User.Search(&model.UserSearch{
			Term:  term,
			Limit: perPage,
		})
	} else {
		users, err = p.client.User.List(&model.UserGetOptions{
			Page:    page,
			PerPage: perPage,
			Active:  true,
		})
	}
	if err != nil {
		p.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	out := make([]map[string]string, 0, len(users))
	for _, u := range users {
		if dto := adminUserDTO(u); dto != nil {
			out = append(out, dto)
		}
	}
	p.writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (p *Plugin) handleAdminListTeams(w http.ResponseWriter, r *http.Request) {
	if !p.requireSystemAdmin(w, r) {
		return
	}
	page, perPage := parsePagination(r, 0, 200)

	teams, err := p.client.Team.List()
	if err != nil {
		p.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	start := min(page*perPage, len(teams))
	end := min(start+perPage, len(teams))
	pageTeams := teams[start:end]

	out := make([]map[string]string, 0, len(pageTeams))
	for _, t := range pageTeams {
		if dto := adminTeamDTO(t); dto != nil {
			out = append(out, dto)
		}
	}
	p.writeJSON(w, http.StatusOK, map[string]any{"teams": out})
}

func (p *Plugin) handleAdminListTeamChannels(w http.ResponseWriter, r *http.Request) {
	if !p.requireSystemAdmin(w, r) {
		return
	}
	teamID := mux.Vars(r)["team_id"]
	if teamID == "" {
		p.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "team_id required"})
		return
	}

	out := make([]map[string]string, 0)
	page := 0
	for {
		list, err := p.client.Channel.ListPublicChannelsForTeam(teamID, page, 200)
		if err != nil {
			p.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if len(list) == 0 {
			break
		}
		for _, ch := range list {
			if dto := adminChannelDTO(ch); dto != nil {
				out = append(out, dto)
			}
		}
		if len(list) < 200 {
			break
		}
		page++
	}
	p.writeJSON(w, http.StatusOK, map[string]any{"channels": out})
}

func (p *Plugin) handleResolveScope(w http.ResponseWriter, r *http.Request) {
	// System-admin-only helper for ScopeEditor: resolve usernames and team:channel to IDs.
	if !p.requireSystemAdmin(w, r) {
		return
	}

	var body struct {
		OrganizerUsername string   `json:"organizer_username"`
		TeamNames         []string `json:"team_names"`
		ChannelSpecs      []string `json:"channel_specs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		p.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	out := map[string]any{}
	if body.OrganizerUsername != "" {
		u, err := p.client.User.GetByUsername(body.OrganizerUsername)
		if err != nil {
			p.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organizer not found"})
			return
		}
		out["organizer"] = map[string]string{"user_id": u.Id, "username": u.Username}
	}

	teams := []map[string]string{}
	for _, name := range body.TeamNames {
		t, err := p.resolveTeamByNameOrDisplay(name)
		if err != nil {
			p.writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "team not found: " + name + " (use the team URL slug, e.g. friends-group, or the exact display name)",
			})
			return
		}
		label := t.DisplayName
		if label == "" {
			label = t.Name
		}
		teams = append(teams, map[string]string{"id": t.Id, "name": label})
	}
	out["teams"] = teams

	channels := []map[string]string{}
	for _, spec := range body.ChannelSpecs {
		parts := splitChannelSpec(spec)
		if len(parts) != 2 {
			continue
		}
		ch, err := p.client.Channel.GetByNameForTeamName(parts[0], parts[1], false)
		if err != nil {
			p.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "channel not found: " + spec})
			return
		}
		channels = append(channels, map[string]string{"id": ch.Id, "team_id": ch.TeamId, "name": ch.DisplayName})
		if channels[len(channels)-1]["name"] == "" {
			channels[len(channels)-1]["name"] = ch.Name
		}
	}
	out["channels"] = channels
	p.writeJSON(w, http.StatusOK, out)
}

func splitChannelSpec(spec string) []string {
	for i := 0; i < len(spec); i++ {
		if spec[i] == ':' {
			return []string{spec[:i], spec[i+1:]}
		}
	}
	return nil
}

// resolveTeamByNameOrDisplay finds a team by URL slug, a spaced display name
// (e.g. "Friends Group" → friends-group), Search, or exact display-name match.
func (p *Plugin) resolveTeamByNameOrDisplay(name string) (*model.Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("empty team name")
	}
	if t, err := p.client.Team.GetByName(name); err == nil && t != nil {
		return t, nil
	}
	slug := strings.ToLower(strings.Join(strings.Fields(name), "-"))
	if slug != "" && !strings.EqualFold(slug, name) {
		if t, err := p.client.Team.GetByName(slug); err == nil && t != nil {
			return t, nil
		}
	}
	if found, err := p.client.Team.Search(name); err == nil {
		for _, t := range found {
			if t == nil {
				continue
			}
			if strings.EqualFold(t.DisplayName, name) || strings.EqualFold(t.Name, name) {
				return t, nil
			}
		}
	}
	all, err := p.client.Team.List()
	if err != nil {
		return nil, err
	}
	for _, t := range all {
		if t == nil {
			continue
		}
		if strings.EqualFold(t.DisplayName, name) || strings.EqualFold(t.Name, name) {
			return t, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func pluginContextIP(r *http.Request) string {
	return r.RemoteAddr
}

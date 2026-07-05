package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/lalbers/mattermost-plugin-community-admin/server/authz"
	"github.com/lalbers/mattermost-plugin-community-admin/server/service"
)

func (p *Plugin) writeJSON(w http.ResponseWriter, status int, v interface{}) {
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
	p.writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":          ctx.ActorID,
		"display_username": ctx.Organizer.DisplayUsername,
		"teams":            ctx.Organizer.Teams,
		"channels":         ctx.Organizer.Channels,
		"permissions":      ctx.Organizer.Permissions,
		"site_url":         ctx.Config.SiteURL,
	})
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

	out := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		teamIDsForUser, _ := p.userService.TeamIDsForUser(u.Id)
		if !checker.UserVisible(orgCtx, teamIDsForUser) {
			continue
		}
		out = append(out, sanitizeUser(u))
	}
	p.writeJSON(w, http.StatusOK, map[string]interface{}{"users": out})
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

	p.writeJSON(w, http.StatusCreated, map[string]interface{}{
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

	p.writeJSON(w, http.StatusOK, map[string]interface{}{
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
	p.writeJSON(w, http.StatusOK, map[string]interface{}{"entries": entries})
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
	p.writeJSON(w, http.StatusOK, map[string]interface{}{"results": results, "dry_run": dryRun})
}

func (p *Plugin) handleResolveScope(w http.ResponseWriter, r *http.Request) {
	// System-admin-only helper for ScopeEditor: resolve usernames and team:channel to IDs.
	actorID := p.actorID(r)
	user, err := p.client.User.Get(actorID)
	if err != nil || !authz.IsSystemAdmin(user.Roles) {
		p.writeJSON(w, http.StatusForbidden, map[string]string{"error": "system admin required"})
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

	out := map[string]interface{}{}
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
		t, err := p.client.Team.GetByName(name)
		if err != nil {
			p.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "team not found: " + name})
			return
		}
		teams = append(teams, map[string]string{"id": t.Id, "name": t.Name})
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
		channels = append(channels, map[string]string{"id": ch.Id, "team_id": ch.TeamId, "name": ch.Name})
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

func pluginContextIP(r *http.Request) string {
	return r.RemoteAddr
}

package main

import (
	"context"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"

	"github.com/lalbers/mattermost-plugin-community-admin/server/command"
	"github.com/lalbers/mattermost-plugin-community-admin/server/service"
)

type clientIPContextKey struct{}

var clientIPKey = clientIPContextKey{}

type Plugin struct {
	plugin.MattermostPlugin

	client *pluginapi.Client

	router *mux.Router

	commandClient command.Command

	configurationLock sync.RWMutex
	configuration     *configuration

	userService       *service.UserService
	membershipService *service.MembershipService
	auditService      *service.AuditService
	rateLimitService  *service.RateLimitService
	batchService      *service.BatchImportService
}

func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)

	if err := p.loadConfiguration(); err != nil {
		return err
	}

	p.userService = service.NewUserService(p.client)
	p.membershipService = service.NewMembershipService(p.client)
	p.auditService = service.NewAuditService(p.client)
	p.rateLimitService = service.NewRateLimitService(p.client)
	p.batchService = service.NewBatchImportService(p.userService, p.membershipService)

	cmd, err := command.NewCommandHandler(p.client, p.getScopeConfig, command.Dependencies{
		UserService:       p.userService,
		MembershipService: p.membershipService,
		AuditService:      p.auditService,
		RateLimitService:  p.rateLimitService,
	})
	if err != nil {
		return err
	}
	p.commandClient = cmd
	p.router = p.initRouter()

	return nil
}

func (p *Plugin) OnConfigurationChange() error {
	if err := p.loadConfiguration(); err != nil {
		return err
	}
	if p.commandClient != nil {
		p.commandClient.SetScopeConfigLoader(p.getScopeConfig)
	}
	return nil
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	response, err := p.commandClient.Handle(args)
	if err != nil {
		return nil, model.NewAppError("ExecuteCommand", "plugin.command.execute_command.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return response, nil
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	// Prefer Mattermost plugin context IPAddress for audit client_ip (no X-Forwarded-* parsing).
	if c != nil && c.IPAddress != "" {
		r = r.WithContext(context.WithValue(r.Context(), clientIPKey, c.IPAddress))
	}
	p.router.ServeHTTP(w, r)
}

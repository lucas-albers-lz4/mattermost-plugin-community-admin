package main

import (
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"

	"github.com/lalbers/mattermost-plugin-community-admin/server/command"
	"github.com/lalbers/mattermost-plugin-community-admin/server/service"
)

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

	p.commandClient = command.NewCommandHandler(p.client, p.getScopeConfig)
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
	p.router.ServeHTTP(w, r)
}

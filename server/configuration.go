package main

import (
	"github.com/pkg/errors"

	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
)

type configuration struct {
	ScopeConfig string `json:"ScopeConfig"`
}

func (c *configuration) Clone() *configuration {
	clone := *c
	return &clone
}

func (p *Plugin) getScopeConfig() *config.ScopeConfig {
	cfg := p.getConfiguration()
	parsed, err := config.ParseScopeConfig(cfg.ScopeConfig)
	if err != nil {
		p.API.LogError("invalid scope config", "error", err.Error())
		return &config.ScopeConfig{Version: config.CurrentVersion, Organizers: []config.Organizer{}}
	}
	return parsed
}

func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()
	p.configuration = configuration
}

func (p *Plugin) loadConfiguration() error {
	configuration := new(configuration)

	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	p.setConfiguration(configuration)

	return nil
}

package main

import (
	"github.com/pkg/errors"

	"github.com/lalbers/mattermost-plugin-community-admin/server/config"
)

type configuration struct {
	ScopeConfig string `json:"ScopeConfig"`
}

// applyParsedScopeConfig returns a newly parsed config, or current on parse
// failure when current is non-nil (last-known-good). On first-load failure it
// returns an empty organizer list.
func applyParsedScopeConfig(current *config.ScopeConfig, raw string) (*config.ScopeConfig, error) {
	parsed, err := config.ParseScopeConfig(raw)
	if err != nil {
		if current != nil {
			return current, err
		}
		return &config.ScopeConfig{Version: config.CurrentVersion, Organizers: []config.Organizer{}}, err
	}
	return parsed, nil
}

func (p *Plugin) getScopeConfig() *config.ScopeConfig {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()
	if p.parsedScopeConfig != nil {
		return p.parsedScopeConfig
	}
	return &config.ScopeConfig{Version: config.CurrentVersion, Organizers: []config.Organizer{}}
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

	raw := ""
	if configuration != nil {
		raw = configuration.ScopeConfig
	}
	next, err := applyParsedScopeConfig(p.parsedScopeConfig, raw)
	if err != nil && p.API != nil {
		p.API.LogError("invalid scope config; keeping previous parsed config if any", "error", err.Error())
	}
	p.parsedScopeConfig = next
}

func (p *Plugin) loadConfiguration() error {
	configuration := new(configuration)

	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	p.setConfiguration(configuration)

	return nil
}

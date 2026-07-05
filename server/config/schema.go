package config

import (
	"encoding/json"
	"fmt"
)

const CurrentVersion = 1

// ScopeConfig is the organizer allowlist stored in the plugin custom setting.
type ScopeConfig struct {
	Version     int         `json:"version"`
	EmailDomain string      `json:"email_domain"`
	SiteURL     string      `json:"site_url"`
	Organizers  []Organizer `json:"organizers"`
}

type Organizer struct {
	UserID             string       `json:"user_id"`
	DisplayUsername    string       `json:"display_username"`
	Teams              []TeamRef    `json:"teams"`
	Channels           []ChannelRef `json:"channels"`
	AllChannelsInTeams []string     `json:"all_channels_in_teams"`
	Permissions        Permissions  `json:"permissions"`
	RateLimits         RateLimits   `json:"rate_limits"`
}

type TeamRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ChannelRef struct {
	ID     string `json:"id"`
	TeamID string `json:"team_id"`
	Name   string `json:"name"`
}

type Permissions struct {
	CreateUser         bool `json:"create_user"`
	EditProfile        bool `json:"edit_profile"`
	ResetPassword      bool `json:"reset_password"`
	ManageMembership   bool `json:"manage_membership"`
	RemoveFromTeam     bool `json:"remove_from_team"`
	DeactivateGlobally bool `json:"deactivate_globally"`
}

type RateLimits struct {
	CreatesPerHour         int `json:"creates_per_hour"`
	PasswordResetsPerHour  int `json:"password_resets_per_hour"`
}

// DefaultPermissions returns safe defaults for a new organizer entry.
func DefaultPermissions() Permissions {
	return Permissions{
		CreateUser:         true,
		EditProfile:        true,
		ResetPassword:      true,
		ManageMembership:   true,
		RemoveFromTeam:     true,
		DeactivateGlobally: false,
	}
}

// DefaultRateLimits returns default rate limits.
func DefaultRateLimits() RateLimits {
	return RateLimits{
		CreatesPerHour:        20,
		PasswordResetsPerHour: 10,
	}
}

// ParseScopeConfig unmarshals and validates scope configuration JSON.
func ParseScopeConfig(raw string) (*ScopeConfig, error) {
	if raw == "" {
		return &ScopeConfig{Version: CurrentVersion, Organizers: []Organizer{}}, nil
	}

	var cfg ScopeConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid scope config JSON: %w", err)
	}

	if cfg.Version == 0 {
		cfg.Version = CurrentVersion
	}
	if cfg.EmailDomain == "" {
		cfg.EmailDomain = "community.local"
	}

	for i := range cfg.Organizers {
		if cfg.Organizers[i].Permissions == (Permissions{}) {
			cfg.Organizers[i].Permissions = DefaultPermissions()
		}
		if cfg.Organizers[i].RateLimits == (RateLimits{}) {
			cfg.Organizers[i].RateLimits = DefaultRateLimits()
		}
	}

	return &cfg, nil
}

// FindOrganizer returns the organizer entry for a user ID.
func (c *ScopeConfig) FindOrganizer(userID string) *Organizer {
	for i := range c.Organizers {
		if c.Organizers[i].UserID == userID {
			return &c.Organizers[i]
		}
	}
	return nil
}

// TeamIDs returns all team IDs for an organizer.
func (o *Organizer) TeamIDs() []string {
	ids := make([]string, 0, len(o.Teams))
	for _, t := range o.Teams {
		ids = append(ids, t.ID)
	}
	return ids
}

// HasTeam returns true if teamID is in the organizer scope.
func (o *Organizer) HasTeam(teamID string) bool {
	for _, t := range o.Teams {
		if t.ID == teamID {
			return true
		}
	}
	return false
}

// HasChannel returns true if channelID is allowed (explicit or wildcard team).
func (o *Organizer) HasChannel(channelID, teamID string) bool {
	for _, ch := range o.Channels {
		if ch.ID == channelID {
			return true
		}
	}
	for _, tid := range o.AllChannelsInTeams {
		if tid == teamID {
			return true
		}
	}
	return false
}

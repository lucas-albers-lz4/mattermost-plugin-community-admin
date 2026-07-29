package service

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

type membershipTeamAPI interface {
	CreateMember(teamID, userID string) (*model.TeamMember, error)
	DeleteMember(teamID, userID, requestorID string) error
}

type membershipChannelAPI interface {
	Get(channelID string) (*model.Channel, error)
	AddMember(channelID, userID string) (*model.ChannelMember, error)
	DeleteMember(channelID, userID string) error
}

// MembershipService manages team and channel membership.
type MembershipService struct {
	teams    membershipTeamAPI
	channels membershipChannelAPI
}

func NewMembershipService(client *pluginapi.Client) *MembershipService {
	return &MembershipService{
		teams:    &client.Team,
		channels: &client.Channel,
	}
}

func newMembershipServiceForTest(teams membershipTeamAPI, channels membershipChannelAPI) *MembershipService {
	return &MembershipService{teams: teams, channels: channels}
}

func (s *MembershipService) AddTeamMember(teamID, userID string) error {
	_, err := s.teams.CreateMember(teamID, userID)
	return err
}

func (s *MembershipService) RemoveTeamMember(teamID, userID, requestorID string) error {
	return s.teams.DeleteMember(teamID, userID, requestorID)
}

func (s *MembershipService) AddChannelMember(channelID, userID string) error {
	_, err := s.channels.AddMember(channelID, userID)
	return err
}

func (s *MembershipService) RemoveChannelMember(channelID, userID string) error {
	return s.channels.DeleteMember(channelID, userID)
}

func (s *MembershipService) GetChannelTeamID(channelID string) (string, error) {
	teamID, _, err := s.GetChannelScope(channelID)
	return teamID, err
}

// GetChannelScope returns the channel's team ID and whether it is a public (open) channel.
func (s *MembershipService) GetChannelScope(channelID string) (teamID string, isOpen bool, err error) {
	ch, err := s.channels.Get(channelID)
	if err != nil {
		return "", false, err
	}
	return ch.TeamId, ch.Type == model.ChannelTypeOpen, nil
}

package service

import (
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

// MembershipService manages team and channel membership.
type MembershipService struct {
	client *pluginapi.Client
}

func NewMembershipService(client *pluginapi.Client) *MembershipService {
	return &MembershipService{client: client}
}

func (s *MembershipService) AddTeamMember(teamID, userID string) error {
	_, err := s.client.Team.CreateMember(teamID, userID)
	return err
}

func (s *MembershipService) RemoveTeamMember(teamID, userID, requestorID string) error {
	return s.client.Team.DeleteMember(teamID, userID, requestorID)
}

func (s *MembershipService) AddChannelMember(channelID, userID string) error {
	_, err := s.client.Channel.AddMember(channelID, userID)
	return err
}

func (s *MembershipService) RemoveChannelMember(channelID, userID string) error {
	return s.client.Channel.DeleteMember(channelID, userID)
}

func (s *MembershipService) GetChannelTeamID(channelID string) (string, error) {
	ch, err := s.client.Channel.Get(channelID)
	if err != nil {
		return "", err
	}
	return ch.TeamId, nil
}

package service

import (
	"errors"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubMembershipTeams struct {
	createErr error
	deleteErr error
	created   []string
	deleted   []string
}

func (s *stubMembershipTeams) CreateMember(teamID, userID string) (*model.TeamMember, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.created = append(s.created, teamID+":"+userID)
	return &model.TeamMember{TeamId: teamID, UserId: userID}, nil
}

func (s *stubMembershipTeams) DeleteMember(teamID, userID, _ string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, teamID+":"+userID)
	return nil
}

type stubMembershipChannels struct {
	byID      map[string]*model.Channel
	getErr    error
	addErr    error
	deleteErr error
	added     []string
	removed   []string
}

func (s *stubMembershipChannels) Get(channelID string) (*model.Channel, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	ch, ok := s.byID[channelID]
	if !ok {
		return nil, errors.New("not found")
	}
	return ch, nil
}

func (s *stubMembershipChannels) AddMember(channelID, userID string) (*model.ChannelMember, error) {
	if s.addErr != nil {
		return nil, s.addErr
	}
	s.added = append(s.added, channelID+":"+userID)
	return &model.ChannelMember{ChannelId: channelID, UserId: userID}, nil
}

func (s *stubMembershipChannels) DeleteMember(channelID, userID string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.removed = append(s.removed, channelID+":"+userID)
	return nil
}

func TestGetChannelScopeOpenAndPrivate(t *testing.T) {
	channels := &stubMembershipChannels{
		byID: map[string]*model.Channel{
			"open":    {Id: "open", TeamId: "t1", Type: model.ChannelTypeOpen},
			"private": {Id: "private", TeamId: "t1", Type: model.ChannelTypePrivate},
		},
	}
	svc := newMembershipServiceForTest(&stubMembershipTeams{}, channels)

	teamID, isOpen, err := svc.GetChannelScope("open")
	require.NoError(t, err)
	assert.Equal(t, "t1", teamID)
	assert.True(t, isOpen)

	teamID, isOpen, err = svc.GetChannelScope("private")
	require.NoError(t, err)
	assert.Equal(t, "t1", teamID)
	assert.False(t, isOpen)
}

func TestGetChannelScopeError(t *testing.T) {
	svc := newMembershipServiceForTest(&stubMembershipTeams{}, &stubMembershipChannels{getErr: errors.New("boom")})
	_, _, err := svc.GetChannelScope("missing")
	require.Error(t, err)
}

func TestMembershipAddRemoveErrors(t *testing.T) {
	teams := &stubMembershipTeams{createErr: errors.New("team fail"), deleteErr: errors.New("team del fail")}
	channels := &stubMembershipChannels{addErr: errors.New("chan fail"), deleteErr: errors.New("chan del fail")}
	svc := newMembershipServiceForTest(teams, channels)

	require.Error(t, svc.AddTeamMember("t1", "u1"))
	require.Error(t, svc.RemoveTeamMember("t1", "u1", "actor"))
	require.Error(t, svc.AddChannelMember("c1", "u1"))
	require.Error(t, svc.RemoveChannelMember("c1", "u1"))
}

func TestMembershipAddRemoveOK(t *testing.T) {
	teams := &stubMembershipTeams{}
	channels := &stubMembershipChannels{}
	svc := newMembershipServiceForTest(teams, channels)

	require.NoError(t, svc.AddTeamMember("t1", "u1"))
	require.NoError(t, svc.RemoveTeamMember("t1", "u1", "actor"))
	require.NoError(t, svc.AddChannelMember("c1", "u1"))
	require.NoError(t, svc.RemoveChannelMember("c1", "u1"))
	assert.Equal(t, []string{"t1:u1"}, teams.created)
	assert.Equal(t, []string{"t1:u1"}, teams.deleted)
	assert.Equal(t, []string{"c1:u1"}, channels.added)
	assert.Equal(t, []string{"c1:u1"}, channels.removed)
}

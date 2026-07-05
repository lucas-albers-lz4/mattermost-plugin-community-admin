package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const (
	auditKeyPrefix   = "audit_"
	auditIndexKey    = "audit_index"
	maxAuditEntries  = 10000
	auditRetention   = 90 * 24 * time.Hour
)

// AuditEntry records an organizer action without secrets.
type AuditEntry struct {
	ID             string            `json:"id"`
	TS             string            `json:"ts"`
	ActorID        string            `json:"actor_id"`
	ActorUsername  string            `json:"actor_username"`
	Action         string            `json:"action"`
	TargetID       string            `json:"target_id,omitempty"`
	TargetUsername string            `json:"target_username,omitempty"`
	TeamID         string            `json:"team_id,omitempty"`
	ChannelID      string            `json:"channel_id,omitempty"`
	ClientIP       string            `json:"client_ip,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type AuditService struct {
	client *pluginapi.Client
}

func NewAuditService(client *pluginapi.Client) *AuditService {
	return &AuditService{client: client}
}

func (s *AuditService) Record(entry AuditEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.TS == "" {
		entry.TS = time.Now().UTC().Format(time.RFC3339)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	key := auditKeyPrefix + entry.ID
	if _, err := s.client.KV.Set(key, data); err != nil {
		return err
	}

	var index []string
	_ = s.client.KV.Get(auditIndexKey, &index)
	index = append([]string{entry.ID}, index...)
	if len(index) > maxAuditEntries {
		removed := index[maxAuditEntries:]
		index = index[:maxAuditEntries]
		for _, id := range removed {
			_ = s.client.KV.Delete(auditKeyPrefix + id)
		}
	}
	_, err = s.client.KV.Set(auditIndexKey, index)
	return err
}

func (s *AuditService) List(limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var index []string
	if err := s.client.KV.Get(auditIndexKey, &index); err != nil {
		return nil, err
	}
	entries := make([]AuditEntry, 0, limit)
	cutoff := time.Now().UTC().Add(-auditRetention)
	for i := 0; i < len(index) && len(entries) < limit; i++ {
		var entry AuditEntry
		if err := s.client.KV.Get(auditKeyPrefix+index[i], &entry); err != nil {
			continue
		}
		ts, err := time.Parse(time.RFC3339, entry.TS)
		if err == nil && ts.Before(cutoff) {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

type RateLimitService struct {
	client *pluginapi.Client
}

func NewRateLimitService(client *pluginapi.Client) *RateLimitService {
	return &RateLimitService{client: client}
}

func (s *RateLimitService) rateKey(actorID, action string) string {
	return fmt.Sprintf("rate_%s_%s_%s", actorID, action, HourBucket())
}

func (s *RateLimitService) CheckAndIncrement(actorID, action string, limit int) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	key := s.rateKey(actorID, action)
	var count int
	_ = s.client.KV.Get(key, &count)
	if count >= limit {
		return false, nil
	}
	count++
	_, err := s.client.KV.Set(key, count)
	return true, err
}

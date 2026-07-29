package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

const (
	auditKeyPrefix  = "audit_"
	auditIndexKey   = "audit_index"
	maxAuditEntries = 10000
	auditRetention  = 90 * 24 * time.Hour
)

var errRateLimited = errors.New("rate limited")

// kvStore is the subset of plugin KV used by audit and rate limiting.
type kvStore interface {
	Get(key string, o any) error
	Set(key string, value any, options ...pluginapi.KVSetOption) (bool, error)
	SetAtomicWithRetries(key string, valueFunc func(oldValue []byte) (newValue any, err error)) error
	Delete(key string) error
}

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
	kv kvStore
}

func NewAuditService(client *pluginapi.Client) *AuditService {
	return &AuditService{kv: &client.KV}
}

func newAuditServiceWithKV(kv kvStore) *AuditService {
	return &AuditService{kv: kv}
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
	if _, err := s.kv.Set(key, data); err != nil {
		return err
	}

	var toDelete []string
	err = s.kv.SetAtomicWithRetries(auditIndexKey, func(oldValue []byte) (any, error) {
		var index []string
		if len(oldValue) > 0 {
			if err := json.Unmarshal(oldValue, &index); err != nil {
				return nil, err
			}
		}
		index = append([]string{entry.ID}, index...)
		toDelete = nil
		if len(index) > maxAuditEntries {
			toDelete = append([]string(nil), index[maxAuditEntries:]...)
			index = index[:maxAuditEntries]
		}
		return index, nil
	})
	if err != nil {
		return err
	}
	for _, id := range toDelete {
		_ = s.kv.Delete(auditKeyPrefix + id)
	}
	return nil
}

func (s *AuditService) List(limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var index []string
	if err := s.kv.Get(auditIndexKey, &index); err != nil {
		return nil, err
	}
	entries := make([]AuditEntry, 0, limit)
	cutoff := time.Now().UTC().Add(-auditRetention)
	for i := 0; i < len(index) && len(entries) < limit; i++ {
		var entry AuditEntry
		if err := s.kv.Get(auditKeyPrefix+index[i], &entry); err != nil {
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
	kv kvStore
}

func NewRateLimitService(client *pluginapi.Client) *RateLimitService {
	return &RateLimitService{kv: &client.KV}
}

func newRateLimitServiceWithKV(kv kvStore) *RateLimitService {
	return &RateLimitService{kv: kv}
}

func (s *RateLimitService) rateKey(actorID, action string) string {
	return fmt.Sprintf("rate_%s_%s_%s", actorID, action, HourBucket())
}

// CheckAndIncrement enforces a per-hour limit atomically.
// limit < 0 means unlimited; limit == 0 denies (callers should resolve 0 via config.Effective*).
func (s *RateLimitService) CheckAndIncrement(actorID, action string, limit int) (bool, error) {
	if limit < 0 {
		return true, nil
	}
	if limit == 0 {
		return false, nil
	}
	key := s.rateKey(actorID, action)
	err := s.kv.SetAtomicWithRetries(key, func(oldValue []byte) (any, error) {
		count := 0
		if len(oldValue) > 0 {
			if err := json.Unmarshal(oldValue, &count); err != nil {
				return nil, err
			}
		}
		if count >= limit {
			return nil, errRateLimited
		}
		return count + 1, nil
	})
	if errors.Is(err, errRateLimited) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

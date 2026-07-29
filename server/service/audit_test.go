package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memKV struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMemKV() *memKV {
	return &memKV{data: map[string][]byte{}}
}

func (m *memKV) Get(key string, o any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, ok := m.data[key]
	if !ok || len(raw) == 0 {
		return nil
	}
	if out, ok := o.(*[]byte); ok {
		*out = append([]byte(nil), raw...)
		return nil
	}
	return json.Unmarshal(raw, o)
}

func (m *memKV) Set(key string, value any, _ ...pluginapi.KVSetOption) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var raw []byte
	var err error
	switch v := value.(type) {
	case []byte:
		raw = append([]byte(nil), v...)
	default:
		raw, err = json.Marshal(value)
		if err != nil {
			return false, err
		}
	}
	m.data[key] = raw
	return true, nil
}

func (m *memKV) SetAtomicWithRetries(key string, valueFunc func(oldValue []byte) (any, error)) error {
	for range 64 {
		m.mu.Lock()
		old := append([]byte(nil), m.data[key]...)
		m.mu.Unlock()

		newVal, err := valueFunc(old)
		if err != nil {
			return err
		}

		m.mu.Lock()
		current := m.data[key]
		if !bytes.Equal(current, old) {
			m.mu.Unlock()
			continue
		}
		var raw []byte
		switch v := newVal.(type) {
		case []byte:
			raw = append([]byte(nil), v...)
		default:
			raw, err = json.Marshal(newVal)
			if err != nil {
				m.mu.Unlock()
				return err
			}
		}
		m.data[key] = raw
		m.mu.Unlock()
		return nil
	}
	return fmt.Errorf("failed to set value after retries")
}

func (m *memKV) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func TestCheckAndIncrementAtomic(t *testing.T) {
	kv := newMemKV()
	svc := newRateLimitServiceWithKV(kv)

	ok, err := svc.CheckAndIncrement("a", "create_user", 2)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = svc.CheckAndIncrement("a", "create_user", 2)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = svc.CheckAndIncrement("a", "create_user", 2)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCheckAndIncrementSemantics(t *testing.T) {
	kv := newMemKV()
	svc := newRateLimitServiceWithKV(kv)

	ok, err := svc.CheckAndIncrement("a", "create_user", -1)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = svc.CheckAndIncrement("a", "create_user", 0)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCheckAndIncrementConcurrent(t *testing.T) {
	kv := newMemKV()
	svc := newRateLimitServiceWithKV(kv)
	const limit = 50
	const goroutines = 80

	var wg sync.WaitGroup
	results := make(chan bool, goroutines)
	for range goroutines {
		wg.Go(func() {
			ok, err := svc.CheckAndIncrement("actor", "create_user", limit)
			require.NoError(t, err)
			results <- ok
		})
	}
	wg.Wait()
	close(results)

	allowed := 0
	for ok := range results {
		if ok {
			allowed++
		}
	}
	assert.Equal(t, limit, allowed)
}

func TestAuditRecordConcurrent(t *testing.T) {
	kv := newMemKV()
	svc := newAuditServiceWithKV(kv)
	const n = 40

	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			require.NoError(t, svc.Record(AuditEntry{
				ActorID:  "a",
				Action:   "create_user",
				TargetID: strconv.Itoa(i),
			}))
		})
	}
	wg.Wait()

	entries, err := svc.List(500)
	require.NoError(t, err)
	assert.Len(t, entries, n)
}

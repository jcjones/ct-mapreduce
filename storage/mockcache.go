package storage

import (
	"fmt"
	"path/filepath" // used for glob-like matching in Keys
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
)

type MockRemoteCache struct {
	Data        map[string][]string
	Expirations map[string]time.Time
}

func NewMockRemoteCache() *MockRemoteCache {
	return &MockRemoteCache{
		Data:        make(map[string][]string),
		Expirations: make(map[string]time.Time),
	}
}

func (ec *MockRemoteCache) CleanupExpiry() {
	now := time.Now()
	for key, timestamp := range ec.Expirations {
		if timestamp.Before(now) {
			delete(ec.Data, key)
			delete(ec.Expirations, key)
		}
	}
}

func (ec *MockRemoteCache) SetInsert(key string, entry string) (bool, error) {
	count := len(ec.Data[key])

	idx := sort.Search(count, func(i int) bool {
		return strings.Compare(entry, ec.Data[key][i]) <= 0
	})

	var cmp int
	if idx < count {
		cmp = strings.Compare(entry, ec.Data[key][idx])
	}

	if idx < count && cmp == 0 {
		glog.V(3).Infof("[%s] Entry already known: %s (pos=%d)", key, entry, idx)
		return false, nil
	}

	// Non-allocating insert, see https://github.com/golang/go/wiki/SliceTricks
	glog.V(3).Infof("[%s] Entry unknown: %s (pos=%d)", key, entry, idx)
	ec.Data[key] = append(ec.Data[key], "")
	copy(ec.Data[key][idx+1:], ec.Data[key][idx:])
	ec.Data[key][idx] = entry
	return true, nil
}

func (ec *MockRemoteCache) SetRemove(key string, entry string) (bool, error) {
	ec.CleanupExpiry()
	count := len(ec.Data[key])

	idx := sort.Search(count, func(i int) bool {
		return strings.Compare(entry, ec.Data[key][i]) <= 0
	})

	var cmp int
	if idx < count {
		cmp = strings.Compare(entry, ec.Data[key][idx])
	}

	if idx < count && cmp == 0 {
		ec.Data[key] = append(ec.Data[key][:idx], ec.Data[key][idx:]...)
		return true, nil
	}

	return false, nil
}

func (ec *MockRemoteCache) SetContains(key string, entry string) (bool, error) {
	ec.CleanupExpiry()
	count := len(ec.Data[key])

	idx := sort.Search(count, func(i int) bool {
		return strings.Compare(entry, ec.Data[key][i]) <= 0
	})

	var cmp int
	if idx < count {
		cmp = strings.Compare(entry, ec.Data[key][idx])
	}

	if idx < count && cmp == 0 {
		return true, nil
	}

	return false, nil
}

func (ec *MockRemoteCache) SetList(key string) ([]string, error) {
	ec.CleanupExpiry()
	return ec.Data[key], nil
}

func (ec *MockRemoteCache) Exists(key string) (bool, error) {
	ec.CleanupExpiry()
	_, ok := ec.Data[key]
	return ok, nil
}

func (ec *MockRemoteCache) ExpireAt(key string, expTime time.Time) error {
	ec.Expirations[key] = expTime
	return nil
}

func (rc *MockRemoteCache) Queue(key string, identifier string) (int64, error) {
	return int64(0), fmt.Errorf("unimplemented")
}

func (rc *MockRemoteCache) Pop(key string) (string, error) {
	return "", fmt.Errorf("unimplemented")
}

func (rc *MockRemoteCache) QueueLength(key string) (int64, error) {
	return int64(0), fmt.Errorf("unimplemented")
}

func (rc *MockRemoteCache) Keys(pattern string) ([]string, error) {
	var matches []string
	for key := range rc.Data {
		matched, err := filepath.Match(pattern, key)
		if err != nil {
			return nil, err
		}
		if matched {
			matches = append(matches, key)
		}
	}

	return matches, nil
}

package storage

import (
	"fmt"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/go-redis/redis"
	"github.com/golang/glog"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr string, cacheTimeout time.Duration) (*RedisCache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:            addr,
		MaxRetries:      10,
		MaxRetryBackoff: 5 * time.Second,
		ReadTimeout:     cacheTimeout,
		WriteTimeout:    cacheTimeout,
	})

	statusr := rdb.Ping()
	if statusr.Err() != nil {
		return nil, statusr.Err()
	}

	rc := &RedisCache{rdb}
	err := rc.MemoryPolicyCorrect()
	if err != nil {
		glog.Warning(err)
	}

	return rc, nil
}

func (rc *RedisCache) MemoryPolicyCorrect() error {
	// maxmemory_policy should be `noeviction`
	confr := rc.client.Info("memory")
	if confr.Err() != nil {
		return confr.Err()
	}
	if strings.Contains(confr.Val(), "maxmemory_policy:noeviction") {
		return nil
	}
	return fmt.Errorf("Redis maxmemory_policy should be `noeviction`. Memory config is set to %s",
		confr.Val())
}

func (rc *RedisCache) SortedInsert(key string, entry string) (bool, error) {
	defer metrics.MeasureSince([]string{"SortedInsert"}, time.Now())
	ir := rc.client.ZAdd(key, redis.Z{
		Score:  0,
		Member: entry,
	})
	added, err := ir.Result()
	if err != nil && strings.HasPrefix(err.Error(), "OOM") {
		glog.Fatalf("Out of memory on Redis insert of entry %s into key %s, error %v", entry, key, err.Error())
	}
	return added == 1, err
}

func (rc *RedisCache) SortedContains(key string, entry string) (bool, error) {
	defer metrics.MeasureSince([]string{"SortedContains"}, time.Now())
	fr := rc.client.ZScore(key, entry)
	if fr.Err() != nil {
		if fr.Err().Error() == "redis: nil" {
			glog.V(3).Infof("Redis does not contain key=%s, entry=%s", key, entry)
			return false, nil
		}
		glog.Warningf("Error at Redis caught, key=%s, entry=%s, err=%+v", key, entry, fr.Err())
		return false, fr.Err()
	}
	return true, nil
}

func (rc *RedisCache) SortedList(key string) ([]string, error) {
	defer metrics.MeasureSince([]string{"SortedList"}, time.Now())
	slicer := rc.client.ZRange(key, 0, -1)
	return slicer.Result()
}

func (rc *RedisCache) Exists(key string) (bool, error) {
	defer metrics.MeasureSince([]string{"Exists"}, time.Now())
	ir := rc.client.Exists(key)
	count, err := ir.Result()
	return count == 1, err
}

func (rc *RedisCache) ExpireAt(key string, aExpTime time.Time) error {
	defer metrics.MeasureSince([]string{"ExpireAt"}, time.Now())
	br := rc.client.ExpireAt(key, aExpTime)
	return br.Err()
}

func (rc *RedisCache) Queue(key string, identifier string) (int64, error) {
	ir := rc.client.RPush(key, identifier)
	return ir.Result()
}

func (rc *RedisCache) Pop(key string) (string, error) {
	sr := rc.client.LPop(key)
	return sr.Result()
}

func (rc *RedisCache) QueueLength(key string) (int64, error) {
	ir := rc.client.LLen(key)
	return ir.Result()
}

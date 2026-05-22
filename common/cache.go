package common

import (
	"sync"
	"time"

	"github.com/Mininglamp-OSS/octo-lib/pkg/redis"
)

// RedisCache redis缓存
type RedisCache struct {
	conn *redis.Conn
}

// NewRedisCache 创建
func NewRedisCache(addr string, password string) *RedisCache {
	r := &RedisCache{}
	r.conn = redis.New(addr, password)
	return r
}

// NewRedisCacheWithConn wraps a caller-built *redis.Conn into a RedisCache.
// Use this when the connection needs options beyond addr/password
// (e.g. TLSConfig built via redis.BuildTLSConfig). Panics if conn is nil
// to surface the misuse immediately rather than at first cache operation.
func NewRedisCacheWithConn(conn *redis.Conn) *RedisCache {
	if conn == nil {
		panic("common.NewRedisCacheWithConn: nil *redis.Conn")
	}
	return &RedisCache{conn: conn}
}

// Set Set
func (r *RedisCache) Set(key string, value string) error {
	return r.conn.Set(key, value)
}

// Delete 删除key
func (r *RedisCache) Delete(key string) error {
	return r.conn.Del(key)
}

// SetAndExpire 包含过期时间
func (r *RedisCache) SetAndExpire(key string, value string, expire time.Duration) error {
	return r.conn.SetAndExpire(key, value, expire)
}

// Get Get
func (r *RedisCache) Get(key string) (string, error) {
	return r.conn.GetString(key)
}

// GetRedisConn 获取redis连接
func (r *RedisCache) GetRedisConn() *redis.Conn {
	return r.conn
}

// MemoryCache 内存缓存
type MemoryCache struct {
	cacheMap map[string]string
	sync.RWMutex
}

// NewMemoryCache 创建
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		cacheMap: map[string]string{},
	}
}

// Set Set
func (m *MemoryCache) Set(key string, value string) error {
	m.Lock()
	defer m.Unlock()
	m.cacheMap[key] = value
	return nil
}

// SetAndExpire SetAndExpire
func (m *MemoryCache) SetAndExpire(key string, value string, expire time.Duration) error {
	m.Lock()
	defer m.Unlock()
	m.cacheMap[key] = value
	return nil
}

// Get Get
func (m *MemoryCache) Get(key string) (string, error) {
	m.RLock()
	defer m.RUnlock()
	return m.cacheMap[key], nil
}

// Delete Delete
func (m *MemoryCache) Delete(key string) error {
	m.Lock()
	defer m.Unlock()
	delete(m.cacheMap, key)
	return nil
}

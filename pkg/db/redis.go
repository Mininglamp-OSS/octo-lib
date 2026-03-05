package db

import "github.com/Mininglamp-OSS/octo-lib/pkg/redis"

func NewRedis(addr string, password string) *redis.Conn {
	return redis.New(addr, password)
}

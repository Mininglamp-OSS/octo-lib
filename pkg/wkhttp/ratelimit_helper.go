package wkhttp

import (
	"os"
	"strconv"

	"github.com/Mininglamp-OSS/octo-lib/pkg/log"
	"go.uber.org/zap"
)

// ParseRPSFromEnv 解析 float 环境变量；缺省或解析失败回退到 def，
// 无效值（负数 / 非法格式）打 Warn 日志，避免操作配置错误静默失败。
func ParseRPSFromEnv(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n <= 0 {
		log.Warn("invalid rate limit env var, using default",
			zap.String("key", key), zap.String("value", v), zap.Float64("default", def))
		return def
	}
	return n
}

// ParseBurstFromEnv 解析 int 环境变量；语义同 ParseRPSFromEnv。
func ParseBurstFromEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Warn("invalid rate limit env var, using default",
			zap.String("key", key), zap.String("value", v), zap.Int("default", def))
		return def
	}
	return n
}

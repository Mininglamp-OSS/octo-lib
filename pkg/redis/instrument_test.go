package redis

import (
	"os"
	"sync"
	"testing"
	"time"

	rd "github.com/go-redis/redis"
)

func TestNormalizeRedisErr(t *testing.T) {
	if normalizeRedisErr(rd.Nil) != nil {
		t.Fatal("redis.Nil should normalize to nil (cache miss is not a failure)")
	}
	other := os.ErrClosed
	if normalizeRedisErr(other) != other {
		t.Fatal("non-Nil error must pass through unchanged")
	}
	if normalizeRedisErr(nil) != nil {
		t.Fatal("nil must stay nil")
	}
}

func TestReportRedisNoObserverIsNoop(t *testing.T) {
	SetRedisObserver(nil)
	reportRedis("get", time.Millisecond, nil) // 不得 panic
}

func TestSetRedisObserverRoundTrip(t *testing.T) {
	var (
		mu  sync.Mutex
		got struct {
			cmd string
			dur time.Duration
			err error
		}
	)
	SetRedisObserver(func(cmd string, dur time.Duration, err error) {
		mu.Lock()
		got.cmd, got.dur, got.err = cmd, dur, err
		mu.Unlock()
	})
	t.Cleanup(func() { SetRedisObserver(nil) })

	reportRedis("set", 3*time.Millisecond, nil)
	mu.Lock()
	defer mu.Unlock()
	if got.cmd != "set" || got.dur != 3*time.Millisecond || got.err != nil {
		t.Fatalf("got cmd=%q dur=%v err=%v", got.cmd, got.dur, got.err)
	}
}

// 集成路径：需要真实 Redis。验证 WrapProcess 把命令名/未命中正确灌给 observer。
func TestInstrumentClientReportsCommands(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set, skipping integration test")
	}

	type sample struct {
		cmd string
		err error
	}
	var (
		mu      sync.Mutex
		samples []sample
	)
	SetRedisObserver(func(cmd string, _ time.Duration, err error) {
		mu.Lock()
		samples = append(samples, sample{cmd, err})
		mu.Unlock()
	})
	t.Cleanup(func() { SetRedisObserver(nil) })

	conn := New(addr, os.Getenv("REDIS_PASSWORD"))
	defer func() { _ = conn.Close() }()

	// GET on a missing key => command name "get", redis.Nil normalized to nil err.
	if _, err := conn.GetString("test:instrument:missing"); err != nil {
		t.Fatalf("GetString: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	var sawGet bool
	for _, s := range samples {
		if s.cmd == "get" {
			sawGet = true
			if s.err != nil {
				t.Fatalf("cache miss must be reported as non-error, got %v", s.err)
			}
		}
	}
	if !sawGet {
		t.Fatalf("expected a 'get' sample, got %+v", samples)
	}
}

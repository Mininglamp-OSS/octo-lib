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
	t.Cleanup(func() { SetRedisObserver(nil) })
	SetRedisObserver(nil)
	reportRedis("get", time.Millisecond, nil) // 不得 panic
}

// 下游 observer panic 必须被接缝兜住,不得顺着 Redis 调用炸上来。
func TestReportRedisRecoversFromObserverPanic(t *testing.T) {
	SetRedisObserver(func(string, time.Duration, error) { panic("boom") })
	t.Cleanup(func() { SetRedisObserver(nil) })
	reportRedis("get", time.Millisecond, nil) // 不得 panic
}

// pipelineErr 的可单测分支:无命令错误时回落到归一化顶层错误。
// (「打头 redis.Nil 盖住后续真实错误」分支需要给命令预置错误,而 go-redis v6
// 未导出 setErr,无法在外部包构造,留给集成路径覆盖。)
func TestPipelineErrFallsBackToTopErr(t *testing.T) {
	okCmd := rd.NewStringCmd("get", "k") // Err()==nil

	if got := pipelineErr([]rd.Cmder{okCmd}, nil); got != nil {
		t.Fatalf("no errors → nil, got %v", got)
	}
	if got := pipelineErr([]rd.Cmder{okCmd}, rd.Nil); got != nil {
		t.Fatalf("top redis.Nil must normalize to nil, got %v", got)
	}
	boom := os.ErrClosed
	if got := pipelineErr([]rd.Cmder{okCmd}, boom); got != boom {
		t.Fatalf("real top err must pass through, got %v", got)
	}
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

// 集成路径：验证导出的 Instrument 能给「裸」*rd.Client 补插桩 —— 即不经
// New/NewWithOptions、需要 Eval/SetNX 等原语而直接构造的客户端(限流/锁/health)。
// 需要真实 Redis。
func TestInstrumentRawClientReportsCommands(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set, skipping integration test")
	}

	var (
		mu     sync.Mutex
		sawGet bool
	)
	SetRedisObserver(func(cmd string, _ time.Duration, _ error) {
		mu.Lock()
		if cmd == "get" {
			sawGet = true
		}
		mu.Unlock()
	})
	t.Cleanup(func() { SetRedisObserver(nil) })

	// 裸客户端,不经 New/NewWithOptions,手动 Instrument。
	client := rd.NewClient(&rd.Options{Addr: addr, Password: os.Getenv("REDIS_PASSWORD")})
	defer func() { _ = client.Close() }()
	Instrument(client)

	if err := client.Get("test:instrument:raw:missing").Err(); err != nil && err != rd.Nil {
		t.Fatalf("Get: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !sawGet {
		t.Fatal("expected a 'get' sample from the manually-instrumented raw client")
	}
}

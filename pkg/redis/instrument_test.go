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

// 这些集成测试共享进程级 observer 单例,勿加 t.Parallel()。

// Instrument(nil) 必须是安全 no-op,不得 panic。
func TestInstrumentNilIsNoop(t *testing.T) {
	Instrument(nil)
}

// 幂等性（非集成,CI 可跑）：对同一 client 连调两次 Instrument,一条命令只应产生一条
// 样本。命令打向无人监听的地址会立即失败,但 WrapProcess 仍按「包了几层」触发对应次数
// 的上报 —— 借此在没有真实 Redis 的情况下探测 hook 是否层叠。
func TestInstrumentIdempotent(t *testing.T) {
	var (
		mu       sync.Mutex
		getCount int
	)
	SetRedisObserver(func(cmd string, _ time.Duration, _ error) {
		mu.Lock()
		if cmd == "get" {
			getCount++
		}
		mu.Unlock()
	})
	t.Cleanup(func() { SetRedisObserver(nil) })

	c := rd.NewClient(&rd.Options{Addr: "127.0.0.1:1", MaxRetries: 0, DialTimeout: 200 * time.Millisecond})
	defer func() { _ = c.Close() }()
	t.Cleanup(func() {
		instrumentedMu.Lock()
		delete(instrumented, c)
		instrumentedMu.Unlock()
	})

	Instrument(c)
	Instrument(c) // 幂等：第二次应为 no-op,不再叠一层 hook

	_ = c.Get("k").Err() // 连接失败会返回 error,但 hook 仍会触发上报

	mu.Lock()
	defer mu.Unlock()
	if getCount != 1 {
		t.Fatalf("Instrument should be idempotent: want 1 'get' sample, got %d (hook stacked?)", getCount)
	}
}

// 集成路径：需要真实 Redis。验证经 New（→Instrument）的 client 把命令名/未命中正确灌给 observer。
func TestNewClientInstrumentedReportsCommands(t *testing.T) {
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

// New/NewWithOptions 走内部 wrapClient,不应把自家 client 登记进全局幂等表 —— 否则
// 短生命周期的 New() client 会被钉住不回收(Jerry-Xin review)。非集成,CI 可跑。
func TestNewDoesNotRetainInGuardMap(t *testing.T) {
	count := func() int {
		instrumentedMu.Lock()
		defer instrumentedMu.Unlock()
		return len(instrumented)
	}
	before := count()
	conn := NewWithOptions(&rd.Options{Addr: "127.0.0.1:1"}) // 惰性,不会真正连接
	defer func() { _ = conn.Close() }()
	if after := count(); after != before {
		t.Fatalf("New/NewWithOptions must not register clients in the idempotence map: len %d -> %d", before, after)
	}
}

// 集成路径：验证导出的 Instrument 能给「裸」*rd.Client 补插桩 —— 即不经
// New/NewWithOptions、需要 Eval/SetNX 等原语而直接构造的客户端(限流/锁/health)。
// 同时验证幂等：连调两次 Instrument,一条 GET 只应产生一条样本（无 hook 层叠/重复计数）。
// 需要真实 Redis。
func TestInstrumentRawClientReportsCommands(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set, skipping integration test")
	}

	var (
		mu       sync.Mutex
		getCount int
	)
	SetRedisObserver(func(cmd string, _ time.Duration, _ error) {
		mu.Lock()
		if cmd == "get" {
			getCount++
		}
		mu.Unlock()
	})
	t.Cleanup(func() { SetRedisObserver(nil) })

	// 裸客户端,不经 New/NewWithOptions,手动 Instrument 两次（幂等：第二次应为 no-op）。
	client := rd.NewClient(&rd.Options{Addr: addr, Password: os.Getenv("REDIS_PASSWORD")})
	defer func() { _ = client.Close() }()
	t.Cleanup(func() { // 别让测试用 client 滞留在全局幂等表里
		instrumentedMu.Lock()
		delete(instrumented, client)
		instrumentedMu.Unlock()
	})
	Instrument(client)
	Instrument(client)

	if err := client.Get("test:instrument:raw:missing").Err(); err != nil && err != rd.Nil {
		t.Fatalf("Get: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if getCount != 1 {
		t.Fatalf("expected exactly 1 'get' sample (idempotent instrument), got %d", getCount)
	}
}

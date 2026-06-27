package redis

import (
	"sync"
	"sync/atomic"
	"time"

	rd "github.com/go-redis/redis"
)

// RedisObserver 接收一次 Redis 命令的耗时。cmd 是命令名（已小写，如 "get"/"set"/
// "eval"），低基数；dur 为耗时；err 为命令结果（nil 表示成功）。
//
// 实现方应保证轻量且不 panic —— 它在每条命令的热路径上同步执行。
//
// octo-lib 不依赖 Prometheus —— 仅暴露这个回调，由 octo-server 在启动时用
// SetRedisObserver 注入真实实现（接 #442 的 DependencyMetrics，label dependency=redis）。
type RedisObserver func(cmd string, dur time.Duration, err error)

// redisObserver 持有进程级 observer。用 atomic.Pointer：启动时 SetRedisObserver
// 设置一次，业务热路径只读；未注入时 reportRedis 为安全 no-op，绝不 panic。
var redisObserver atomic.Pointer[RedisObserver]

// SetRedisObserver 注入进程级 Redis observer。传 nil 可清除（恢复 no-op）。
// 通常在 main 启动早期调用一次。
//
// 注意：observer 是进程级单例。SetRedisObserver 影响之后经 Instrument 计时的所有
// client 的上报目标；Instrument 自身在 New/NewWithOptions 构造时（或裸 client 手动
// 调用时）即已挂好 hook，与 observer 是否注入无关。
func SetRedisObserver(o RedisObserver) {
	if o == nil {
		redisObserver.Store(nil)
		return
	}
	redisObserver.Store(&o)
}

// reportRedis 把一次命令转发给已注入的 observer；未注入时为 no-op。
//
// 在接缝处兜住下游 observer 的 panic:observer 跑在每条命令的热路径上,一个
// nil-deref 不该顺着 Redis 调用炸上来,最多丢掉这一条埋点样本。
func reportRedis(cmd string, dur time.Duration, err error) {
	defer func() { _ = recover() }()
	if p := redisObserver.Load(); p != nil {
		(*p)(cmd, dur, err)
	}
}

// instrumented 标记已经过 Instrument(公开入口)挂过 hook 的 client,使其幂等:
// 重复调用是 no-op。go-redis v6 的 WrapProcess 每调一次就在上一层外再包一层,没有
// 这道 guard,对同一 client 调两次会让每条命令重复计数 2×,且无运行期信号 —— ~15 个
// 裸 client 调用方场景下极易误触。
//
// 仅公开的 Instrument 登记此表;New/NewWithOptions 走内部 wrapClient,不登记 —— 它们
// 对刚构造、必然只插一次的 client 操作,无重复风险,也就不必把(可能短生命周期的)
// New 构造 client 钉在表里不被 GC。所以本表只持有调用方显式传入的长生命周期裸 client。
var (
	instrumentedMu sync.Mutex
	instrumented   = map[*rd.Client]struct{}{}
)

// wrapClient 把每条命令的计时 hook 挂到 client 上(WrapProcess/WrapProcessPipeline)。
// 这是内部低层实现,不做 nil / 幂等 guard —— 供 New/NewWithOptions 对自家刚构造的
// client 调用。公开的 Instrument 在其之上叠加 nil + 幂等 guard。
func wrapClient(client *rd.Client) {
	client.WrapProcess(func(old func(rd.Cmder) error) func(rd.Cmder) error {
		return func(cmd rd.Cmder) error {
			start := time.Now()
			err := old(cmd)
			reportRedis(cmd.Name(), time.Since(start), normalizeRedisErr(err))
			return err
		}
	})
	client.WrapProcessPipeline(func(old func([]rd.Cmder) error) func([]rd.Cmder) error {
		return func(cmds []rd.Cmder) error {
			start := time.Now()
			err := old(cmds)
			reportRedis("pipeline", time.Since(start), pipelineErr(cmds, err))
			return err
		}
	})
}

// Instrument 给一个 go-redis v6 的 *rd.Client 挂上每条命令的计时 hook,使其命令也灌入
// 已注册的 RedisObserver —— 与经 New/NewWithOptions 构造的客户端完全一致。
//
// 用途:为**直接用 rd.NewClient 构造的裸客户端**补插桩 —— 它们往往需要 Eval/Script/
// SetNX 等 pkg/redis 的 Conn 包装未暴露的原语(如限流令牌桶、OIDC 锁、health 探针),
// 因而绕过了 New/NewWithOptions 的自动插桩。调用方在构造后调一次本函数即可纳入
// dependency=redis 指标。
//
// 时机:必须在 client 被共享 / 发起命令**之前**调用。go-redis v6 的 WrapProcess 赋值
// 未加锁,与在途命令并发会 race。
//
// 幂等且 nil-safe:client 为 nil 时直接返回;对同一 client 重复调用是 no-op,不会层叠
// hook、不会重复计数。注意它会在内部表里持有该 client 的引用(见 instrumented),故面向
// 长生命周期客户端(启动期单例),不要在热循环里对临时 client 反复调用。
//
// 计时细节:
//   - 单条命令：op = cmd.Name()；
//   - pipeline：作为一次往返整体上报 op = "pipeline"（管道内多命令共享一次 RTT，
//     无法拆分单命令耗时）。
//
// redis.Nil（key 不存在 / 无匹配）是正常的「未命中」语义而非故障，归一为非错误，
// 避免把命中率噪声混进 error 状态。WrapProcess 只观测、不改变返回给调用方的 err。
func Instrument(client *rd.Client) {
	if client == nil {
		return
	}
	instrumentedMu.Lock()
	if _, ok := instrumented[client]; ok {
		instrumentedMu.Unlock()
		return
	}
	instrumented[client] = struct{}{}
	instrumentedMu.Unlock()

	wrapClient(client)
}

// normalizeRedisErr 把 redis.Nil 归一为 nil（未命中不是故障），其余原样返回。
func normalizeRedisErr(err error) error {
	if err == rd.Nil {
		return nil
	}
	return err
}

// pipelineErr 提炼一个 pipeline 批次里「真正的」错误用于打点。go-redis v6 的
// pipeline 顶层错误是 cmdsFirstErr —— 按顺序取第一条命令的错误,因此一个打头的
// redis.Nil（例如对不存在 key 的 GET）会把后面命令的真实失败盖住,导致整批被记成
// 成功。这里扫描各命令,返回第一条非 redis.Nil 的命令错误;若只有 redis.Nil / 无
// 命令错误,再回落到归一化后的顶层错误。
func pipelineErr(cmds []rd.Cmder, topErr error) error {
	for _, cmd := range cmds {
		if e := cmd.Err(); e != nil && e != rd.Nil {
			return e
		}
	}
	return normalizeRedisErr(topErr)
}

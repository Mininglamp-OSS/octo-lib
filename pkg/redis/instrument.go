package redis

import (
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
// 注意：observer 是进程级单例。SetRedisObserver 影响之后经 instrumentClient 计时
// 的所有 client 的上报目标；instrumentClient 自身在 New/NewWithOptions 构造时即已
// 挂好 hook，与 observer 是否注入无关。
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

// instrumentClient 用 go-redis v6 的 WrapProcess/WrapProcessPipeline 给每条命令计时。
// go-redis v6 没有 v8 的 AddHook，WrapProcess 是其等价物（包裹 process 函数）。
//
//   - 单条命令：op = cmd.Name()；
//   - pipeline：作为一次往返整体上报 op = "pipeline"（管道内多命令共享一次 RTT，
//     无法拆分单命令耗时）。
//
// redis.Nil（key 不存在 / 无匹配）是正常的「未命中」语义而非故障，归一为非错误，
// 避免把命中率噪声混进 error 状态。WrapProcess 只观测、不改变返回给调用方的 err。
func instrumentClient(client *rd.Client) {
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

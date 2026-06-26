package db

import (
	"context"
	"database/sql/driver"
	"sync/atomic"
	"time"

	"github.com/gocraft/dbr/v2"
)

// DBObserver 接收一次 MySQL 客户端调用的耗时。op 是低基数枚举：
//
//   - "connect"：建立一条新连接（驱动握手 + 鉴权），由 instrumentedConnector 上报，
//     用于把「建连握手」从「取连接等待 / 执行」中隔离出来；
//   - "query"：一条 dbr 查询（select / exec）的执行耗时，由 metricEventReceiver 上报。
//
// dur 为本次调用耗时；err 为调用结果（nil 表示成功）。实现方应保证轻量且不 panic，
// 它在每条 DB 调用的热路径上同步执行。
//
// octo-lib 不依赖 Prometheus —— 仅暴露这个回调，由 octo-server 在启动时用
// SetDBObserver 注入真实实现（接 #442 的 DependencyMetrics，label dependency=mysql）。
type DBObserver func(op string, dur time.Duration, err error)

// dbObserver 持有进程级 observer。用 atomic.Pointer：启动时 SetDBObserver 设置一次，
// 业务热路径只读；未注入（指标关闭 / 单测）时 reportDB 为安全 no-op，绝不 panic。
var dbObserver atomic.Pointer[DBObserver]

// SetDBObserver 注入进程级 DB observer。传 nil 可清除（恢复 no-op）。
// 通常在 main 启动早期调用一次。
func SetDBObserver(o DBObserver) {
	if o == nil {
		dbObserver.Store(nil)
		return
	}
	dbObserver.Store(&o)
}

// reportDB 把一次调用转发给已注入的 observer；未注入时为 no-op。
func reportDB(op string, dur time.Duration, err error) {
	if p := dbObserver.Load(); p != nil {
		(*p)(op, dur, err)
	}
}

// metricEventReceiver 实现 dbr.EventReceiver，把每条查询的执行耗时经
// Timing/TimingKv 上报为 op="query"。其余事件（Event*/EventErr*）由内嵌的
// NullEventReceiver 兜底为 no-op。
//
// 为何只在 Timing 上报、不在 EventErr 上报：dbr 的 select/exec 用 defer 触发
// TimingKv，因此「失败的查询」会同时触发 TimingKv（带真实耗时）和 EventErrKv。
// 若在两处都上报，一条失败查询会产生两条样本（一条 ok + 一条 error），既重复计数
// 又把失败误标成 ok。故只取 Timing 这个「每条查询必触发一次」的信号，统一记为耗时
// 样本。代价：本指标不区分查询成功/失败（错误另由 dbr 自身日志与 connect 段体现），
// 与发版说明里的 caveat 一致。
type metricEventReceiver struct {
	*dbr.NullEventReceiver
}

// Timing 接收一条查询的执行耗时（纳秒），上报为 op="query"。
func (metricEventReceiver) Timing(eventName string, nanoseconds int64) {
	reportDB("query", time.Duration(nanoseconds), nil)
}

// TimingKv 接收一条查询的执行耗时（纳秒）及附带 kv，上报为 op="query"。
// kv（含 sql 文本）刻意不进 label，避免基数爆炸。
func (metricEventReceiver) TimingKv(eventName string, nanoseconds int64, kvs map[string]string) {
	reportDB("query", time.Duration(nanoseconds), nil)
}

// instrumentedConnector 包一层 driver.Connector，对每次建连（Connect）计时并上报
// op="connect"，从而把建连握手从执行耗时中隔离出来。Driver() 由内嵌 Connector 提升。
type instrumentedConnector struct {
	driver.Connector
}

// Connect 计时底层建连并上报。失败的建连同样带耗时（握手/鉴权失败也是真实耗时），
// 故 connect 段保留完整的 ok/error 状态。
func (c instrumentedConnector) Connect(ctx context.Context) (driver.Conn, error) {
	start := time.Now()
	conn, err := c.Connector.Connect(ctx)
	reportDB("connect", time.Since(start), err)
	return conn, err
}

package db

import (
	"context"
	"database/sql/driver"
	"sync/atomic"
	"time"
)

// DBObserver 接收一次 MySQL 客户端调用的耗时。op 是低基数枚举：
//
//   - "connect"：建立一条新连接（驱动握手 + 鉴权），由 instrumentedConnector 上报；
//   - "query"：拿到连接之后一条语句的纯执行耗时（ExecContext / QueryContext），由
//     instrumentedConn 上报。
//
// 关键：query 在连接已从池中取得之后才开始计时,因此「建连握手 / 取连接等待 / 执行」
// 三段不重叠 —— connect=握手,取连接等待看连接池的 WaitDuration,query=执行。
//
// dur 为本次调用耗时；err 为调用结果（nil 表示成功）。connect 与 query 都携带真实
// err（query 走驱动层,拿得到执行错误）。实现方应保证轻量且不 panic，它在每条 DB
// 调用的热路径上同步执行。
//
// octo-lib 不依赖 Prometheus —— 仅暴露这个回调，由 octo-server 在启动时用
// SetDBObserver 注入真实实现（接 #442 的 DependencyMetrics，label dependency=mysql）。
//
// 覆盖范围：dbr 通过插值把参数内联进 SQL 后直接走 ExecContext/QueryContext,故其全部
// 查询都被计时。显式预编译语句（driver.Stmt）的执行走 Stmt 而非 conn,不在此覆盖
// （dbr 不预编译）。
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
//
// 在接缝处兜住下游 observer 的 panic:observer 跑在每条 DB 调用的热路径上,一个
// nil-deref 不该顺着所有查询炸上来,最多丢掉这一条埋点样本。godoc 已要求实现方
// "不 panic",这里再加一道防线而非纯靠信任。
func reportDB(op string, dur time.Duration, err error) {
	defer func() { _ = recover() }()
	if p := dbObserver.Load(); p != nil {
		(*p)(op, dur, err)
	}
}

// instrumentedConnector 包一层 driver.Connector：对每次建连（Connect）计时并上报
// op="connect"，并把返回的连接包成 instrumentedConn 以便对执行计时。Driver() 由内嵌
// Connector 提升。
type instrumentedConnector struct {
	driver.Connector
}

// Connect 计时底层建连并上报。失败的建连同样带耗时（握手/鉴权失败也是真实耗时），
// 故 connect 段保留完整的 ok/error 状态。成功则把连接包成 instrumentedConn。
func (c instrumentedConnector) Connect(ctx context.Context) (driver.Conn, error) {
	start := time.Now()
	conn, err := c.Connector.Connect(ctx)
	reportDB("connect", time.Since(start), err)
	if err != nil {
		return nil, err
	}
	return instrumentedConn{conn}, nil
}

// instrumentedConn 包一层 driver.Conn，对 ExecContext/QueryContext 计时并上报
// op="query"。连接此时已从池中取得,测的是纯执行耗时(不含取连接等待/握手)。
//
// database/sql 通过类型断言探测 driver.Conn 的可选接口。用接口类型内嵌 driver.Conn
// 只会提升 Prepare/Close/Begin,其余可选接口（ExecerContext / QueryerContext /
// ConnPrepareContext / ConnBeginTx / Pinger / NamedValueChecker / SessionResetter /
// Validator）会被「藏掉」,导致 database/sql 退化(丢预编译上下文 / 连接复用 reset /
// 参数检查 / 坏连接探活等)。因此这里把 go-sql-driver/mysql 实现的全部可选接口逐一
// 重新暴露并委托;只有 Exec/Query 两条加计时。断言失败时给出与「未实现」等价的安全
// 回退(对 mysql 而言是 dead path,纯防御)。
type instrumentedConn struct {
	driver.Conn
}

var (
	_ driver.ExecerContext      = instrumentedConn{}
	_ driver.QueryerContext     = instrumentedConn{}
	_ driver.ConnPrepareContext = instrumentedConn{}
	_ driver.ConnBeginTx        = instrumentedConn{}
	_ driver.Pinger             = instrumentedConn{}
	_ driver.NamedValueChecker  = instrumentedConn{}
	_ driver.SessionResetter    = instrumentedConn{}
	_ driver.Validator          = instrumentedConn{}
)

// ExecContext 计执行耗时并上报 op="query"。driver.ErrSkip 表示「请退回 Prepare 路径」,
// 不是真实执行,不计样本。
func (c instrumentedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	ec, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	start := time.Now()
	res, err := ec.ExecContext(ctx, query, args)
	if err != driver.ErrSkip {
		reportDB("query", time.Since(start), err)
	}
	return res, err
}

// QueryContext 计执行耗时并上报 op="query"。耗时覆盖到结果集首包返回（执行+首响应）,
// 不含调用方后续逐行读取。
func (c instrumentedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	qc, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	start := time.Now()
	rows, err := qc.QueryContext(ctx, query, args)
	if err != driver.ErrSkip {
		reportDB("query", time.Since(start), err)
	}
	return rows, err
}

// 以下为可选接口的透传委托(不计时),用于在包装后保持底层驱动的完整能力。

func (c instrumentedConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if cpc, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return cpc.PrepareContext(ctx, query)
	}
	return c.Prepare(query)
}

func (c instrumentedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if cbt, ok := c.Conn.(driver.ConnBeginTx); ok {
		return cbt.BeginTx(ctx, opts)
	}
	return c.Conn.Begin() //nolint:staticcheck // 非 ctx 驱动的防御回退;mysql 恒实现 ConnBeginTx
}

func (c instrumentedConn) Ping(ctx context.Context) error {
	if p, ok := c.Conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func (c instrumentedConn) CheckNamedValue(nv *driver.NamedValue) error {
	if nvc, ok := c.Conn.(driver.NamedValueChecker); ok {
		return nvc.CheckNamedValue(nv)
	}
	return driver.ErrSkip // 退回 database/sql 默认参数转换
}

func (c instrumentedConn) ResetSession(ctx context.Context) error {
	if sr, ok := c.Conn.(driver.SessionResetter); ok {
		return sr.ResetSession(ctx)
	}
	return nil
}

func (c instrumentedConn) IsValid() bool {
	if v, ok := c.Conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

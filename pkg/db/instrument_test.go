package db

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"
)

// capturedDB 记录 observer 收到的一次调用。
type capturedDB struct {
	op  string
	dur time.Duration
	err error
	n   int
}

// installDBObserver 安装一个记录型 observer 并在测试结束时清除。
func installDBObserver(t *testing.T) *capturedDB {
	t.Helper()
	c := &capturedDB{}
	SetDBObserver(func(op string, dur time.Duration, err error) {
		c.op, c.dur, c.err = op, dur, err
		c.n++
	})
	t.Cleanup(func() { SetDBObserver(nil) })
	return c
}

// stubConn 实现 driver.Conn + ExecerContext/QueryerContext,用于测 instrumentedConn。
type stubConn struct {
	execErr  error
	queryErr error
}

func (stubConn) Prepare(string) (driver.Stmt, error) { return nil, nil }
func (stubConn) Close() error                        { return nil }
func (stubConn) Begin() (driver.Tx, error)           { return nil, nil }
func (c stubConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return nil, c.execErr
}
func (c stubConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return nil, c.queryErr
}

func TestInstrumentedConnReportsQuerySuccess(t *testing.T) {
	c := installDBObserver(t)
	ic := instrumentedConn{stubConn{}}

	if _, err := ic.ExecContext(context.Background(), "update x", nil); err != nil {
		t.Fatalf("ExecContext: %v", err)
	}
	if c.n != 1 || c.op != "query" || c.err != nil {
		t.Fatalf("exec: got op=%q err=%v n=%d", c.op, c.err, c.n)
	}

	if _, err := ic.QueryContext(context.Background(), "select 1", nil); err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	if c.n != 2 || c.op != "query" || c.err != nil {
		t.Fatalf("query: got op=%q err=%v n=%d", c.op, c.err, c.n)
	}
}

// 驱动层拿得到真实执行错误,故 op="query" 现在携带 error 状态(对比旧 dbr.Timing 路径)。
func TestInstrumentedConnReportsQueryError(t *testing.T) {
	c := installDBObserver(t)
	boom := errors.New("syntax error")
	ic := instrumentedConn{stubConn{execErr: boom}}

	_, err := ic.ExecContext(context.Background(), "bad sql", nil)
	if !errors.Is(err, boom) {
		t.Fatalf("err should propagate, got %v", err)
	}
	if c.n != 1 || c.op != "query" || !errors.Is(c.err, boom) {
		t.Fatalf("failed query must report with err: op=%q err=%v n=%d", c.op, c.err, c.n)
	}
}

// driver.ErrSkip 是「退回 Prepare 路径」的信号,不是真实执行,不应计样本。
func TestInstrumentedConnExecErrSkipNotReported(t *testing.T) {
	c := installDBObserver(t)
	ic := instrumentedConn{stubConn{execErr: driver.ErrSkip}}

	if _, err := ic.ExecContext(context.Background(), "x", nil); err != driver.ErrSkip {
		t.Fatalf("ErrSkip should propagate, got %v", err)
	}
	if c.n != 0 {
		t.Fatalf("ErrSkip must not be reported as a query sample, got n=%d", c.n)
	}
}

// stubConnector 是一个可控返回的 driver.Connector，用于测 instrumentedConnector。
type stubConnector struct {
	conn driver.Conn
	err  error
}

func (s stubConnector) Connect(context.Context) (driver.Conn, error) { return s.conn, s.err }
func (s stubConnector) Driver() driver.Driver                        { return nil }

func TestInstrumentedConnectorReportsConnectSuccess(t *testing.T) {
	c := installDBObserver(t)
	ic := instrumentedConnector{Connector: stubConnector{conn: nil, err: nil}}

	_, err := ic.Connect(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if c.n != 1 || c.op != "connect" || c.err != nil {
		t.Fatalf("got op=%q err=%v n=%d", c.op, c.err, c.n)
	}
}

func TestInstrumentedConnectorReportsConnectError(t *testing.T) {
	c := installDBObserver(t)
	sentinel := errors.New("dial fail")
	ic := instrumentedConnector{Connector: stubConnector{err: sentinel}}

	_, err := ic.Connect(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("err should propagate, got %v", err)
	}
	if c.n != 1 || c.op != "connect" || !errors.Is(c.err, sentinel) {
		t.Fatalf("failed connect must report with err: op=%q err=%v n=%d", c.op, c.err, c.n)
	}
}

// 未注入 observer 时上报必须是安全 no-op。
func TestReportDBNoObserverIsNoop(t *testing.T) {
	t.Cleanup(func() { SetDBObserver(nil) })
	SetDBObserver(nil)
	reportDB("query", time.Millisecond, nil) // 不得 panic
}

// 下游 observer panic 必须被接缝兜住,不得顺着 DB 调用炸上来。
func TestReportDBRecoversFromObserverPanic(t *testing.T) {
	SetDBObserver(func(string, time.Duration, error) { panic("boom") })
	t.Cleanup(func() { SetDBObserver(nil) })
	reportDB("query", time.Millisecond, nil) // 不得 panic
}

// NewMySQL 不应在构造时建连（sql.OpenDB 是惰性的），故无需真实 DB 即可构造成功。
func TestNewMySQLConstructsWithoutConnecting(t *testing.T) {
	sess := NewMySQL("user:pass@tcp(127.0.0.1:3306)/db?parseTime=true", 10, 5, time.Hour)
	if sess == nil || sess.Connection == nil || sess.DB == nil {
		t.Fatal("NewMySQL returned an incomplete session")
	}
}

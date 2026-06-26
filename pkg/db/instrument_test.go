package db

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/gocraft/dbr/v2"
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

func TestMetricEventReceiverTimingReportsQuery(t *testing.T) {
	c := installDBObserver(t)
	r := metricEventReceiver{&dbr.NullEventReceiver{}}

	r.Timing("dbr.select", int64(5*time.Millisecond))
	if c.n != 1 || c.op != "query" || c.err != nil || c.dur != 5*time.Millisecond {
		t.Fatalf("Timing: got op=%q dur=%v err=%v n=%d", c.op, c.dur, c.err, c.n)
	}

	r.TimingKv("dbr.exec", int64(7*time.Millisecond), map[string]string{"sql": "update x"})
	if c.n != 2 || c.op != "query" || c.err != nil || c.dur != 7*time.Millisecond {
		t.Fatalf("TimingKv: got op=%q dur=%v err=%v n=%d", c.op, c.dur, c.err, c.n)
	}
}

// errEvents 断言失败路径只走 Event*（no-op，不上报），不产生 query 样本 ——
// 印证「失败查询不会在 Timing 之外被二次上报」的设计。
func TestMetricEventReceiverErrorPathDoesNotReport(t *testing.T) {
	c := installDBObserver(t)
	r := metricEventReceiver{&dbr.NullEventReceiver{}}

	sentinel := errors.New("boom")
	if got := r.EventErrKv("dbr.exec.exec", sentinel, nil); got != sentinel {
		t.Fatalf("EventErrKv should passthrough err, got %v", got)
	}
	r.Event("dbr.begin")
	if c.n != 0 {
		t.Fatalf("error/event path must not report, got n=%d", c.n)
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

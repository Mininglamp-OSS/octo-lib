package config

import (
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/sendgrid/rest"
)

// 未注入 observer 时上报必须是安全 no-op，不得 panic（指标关闭 / 单测默认路径）。
func TestReportIMNoObserverIsNoop(t *testing.T) {
	t.Cleanup(func() { SetIMObserver(nil) })
	SetIMObserver(nil)
	reportIM(opSendMessage, time.Millisecond, nil) // 不得 panic
}

// 下游 observer panic 必须被接缝兜住，不得顺着 IM 调用炸上来。
func TestReportIMRecoversFromObserverPanic(t *testing.T) {
	SetIMObserver(func(string, time.Duration, error) { panic("boom") })
	t.Cleanup(func() { SetIMObserver(nil) })
	reportIM(opSendMessage, time.Millisecond, nil) // 不得 panic
}

// 注入的 observer 必须原样收到 op / dur / err。
func TestSetIMObserverRoundTrip(t *testing.T) {
	var (
		mu    sync.Mutex
		calls int
		got   struct {
			op  string
			dur time.Duration
			err error
		}
	)
	SetIMObserver(func(op string, dur time.Duration, err error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		got.op, got.dur, got.err = op, dur, err
	})
	t.Cleanup(func() { SetIMObserver(nil) })

	sentinel := errors.New("upstream failure")
	reportIM(opSendMessage, 5*time.Millisecond, sentinel)

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("observer 应被调用一次，实际 %d", calls)
	}
	if got.op != opSendMessage || got.dur != 5*time.Millisecond || got.err != sentinel {
		t.Fatalf("observer 收到参数不符: %+v", got)
	}
}

// imCallErr 的分类：传输错误透传；无响应 / 非 200 归为 errIMBadStatus；200 为成功(nil)。
// 口径与 handlerIMError 一致（4xx 也算失败），保证指标 status 与方法自身对成功的定义同源。
func TestIMCallErr(t *testing.T) {
	transport := errors.New("dial timeout")
	if got := imCallErr(nil, transport); got != transport {
		t.Fatalf("传输错误必须透传，got %v", got)
	}
	if got := imCallErr(nil, nil); got != errIMBadStatus {
		t.Fatalf("nil 响应必须归为 bad status，got %v", got)
	}
	if got := imCallErr(&rest.Response{StatusCode: http.StatusBadRequest}, nil); got != errIMBadStatus {
		t.Fatalf("4xx 必须归为 bad status，got %v", got)
	}
	if got := imCallErr(&rest.Response{StatusCode: http.StatusInternalServerError}, nil); got != errIMBadStatus {
		t.Fatalf("5xx 必须归为 bad status，got %v", got)
	}
	if got := imCallErr(&rest.Response{StatusCode: http.StatusOK}, nil); got != nil {
		t.Fatalf("200 必须为成功(nil)，got %v", got)
	}
}

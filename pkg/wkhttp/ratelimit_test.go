package wkhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	rd "github.com/go-redis/redis"
)

// newTestRedis 连接到 testenv 上的 Redis 容器（端口 6399）。若不可用则跳过测试，
// 与 octo-server 内的原版 ratelimit 测试约定一致；CI 容器有 Redis 时才执行。
func newTestRedis(t *testing.T) *rd.Client {
	t.Helper()
	c := rd.NewClient(&rd.Options{Addr: "127.0.0.1:6399"})
	if err := c.Ping().Err(); err != nil {
		t.Skipf("testenv redis unavailable at 127.0.0.1:6399: %v", err)
	}
	cleanRateLimitKeys(t, c)
	t.Cleanup(func() {
		cleanRateLimitKeys(t, c)
		_ = c.Close()
	})
	return c
}

func cleanRateLimitKeys(t *testing.T, c *rd.Client) {
	t.Helper()
	keys, err := c.Keys("ratelimit:*").Result()
	if err != nil {
		t.Fatalf("scan ratelimit keys: %v", err)
	}
	if len(keys) == 0 {
		return
	}
	if err := c.Del(keys...).Err(); err != nil {
		t.Fatalf("del ratelimit keys: %v", err)
	}
}

// TestRateLimitErrorSpec 验证三个限流中间件共用的 ErrorSpec 形状：
// Code 是 i18n key，retry_after 通过 Details 透传给 renderer。
func TestRateLimitErrorSpec(t *testing.T) {
	spec := rateLimitErrorSpec(7)
	if spec.Code != "err.shared.rate.limited" {
		t.Errorf("Code=%q", spec.Code)
	}
	if spec.TransportStatus != http.StatusTooManyRequests {
		t.Errorf("TransportStatus=%d", spec.TransportStatus)
	}
	if got, _ := spec.Details["retry_after"].(int); got != 7 {
		t.Errorf("Details[retry_after]=%v", spec.Details["retry_after"])
	}
	if spec.DefaultMessage == "" {
		t.Error("DefaultMessage empty — would break fallback envelope")
	}
}

// TestRateLimitMiddlewareDeniesViaRenderer 端到端验证：burst=1 让第二次请求拒绝，
// 注入 stubRenderer 后能拿到 err.shared.rate.limited code。需要 Redis。
func TestRateLimitMiddlewareDeniesViaRenderer(t *testing.T) {
	client := newTestRedis(t)
	wk := New()
	stub := &stubRenderer{}
	wk.SetErrorRenderer(stub)

	mw := wk.RateLimitMiddleware(context.Background(), client, 0.1, 1)
	wk.Use(mw)
	wk.GET("/", noopHandler)

	// 首次：放行
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("X-Real-Ip", "1.2.3.4")
	wk.ServeHTTP(w1, req1)

	// 第二次：burst 耗尽，拒绝
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Real-Ip", "1.2.3.4")
	wk.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status=%d, want 429", w2.Code)
	}
	if stub.lastSpec().Code != "err.shared.rate.limited" {
		t.Errorf("renderer not invoked with rate-limit spec: got %q", stub.lastSpec().Code)
	}
	if got, _ := stub.lastSpec().Details["retry_after"].(int); got <= 0 {
		t.Errorf("retry_after = %v, want >0", stub.lastSpec().Details["retry_after"])
	}
}

// TestRateLimitMiddlewareDefaultFallback 未注入 renderer 时拒绝路径输出
// 历史 envelope {msg:"请求过于频繁，请稍后再试", status:429}。
func TestRateLimitMiddlewareDefaultFallback(t *testing.T) {
	client := newTestRedis(t)
	wk := New()
	mw := wk.StrictIPRateLimitMiddleware(context.Background(), client, "test", 0.1, 1)
	wk.GET("/", mw, noopHandler)

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("X-Real-Ip", "5.6.7.8")
	wk.ServeHTTP(w1, req1)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Real-Ip", "5.6.7.8")
	wk.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", w2.Code, w2.Body.String())
	}
	if !contains(w2.Body.String(), "请求过于频繁") {
		t.Errorf("body=%q, want legacy zh message", w2.Body.String())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

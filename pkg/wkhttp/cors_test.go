package wkhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCORSMiddlewareAdvertisesI18nHeaders 验证 i18n 协议要求的三个请求头
// 出现在 Access-Control-Allow-Headers，两个响应头出现在 Access-Control-Expose-Headers。
// 这是 issue 46 CORS 校准的核心验收。
func TestCORSMiddlewareAdvertisesI18nHeaders(t *testing.T) {
	wk := New()
	wk.Use(CORSMiddleware())
	wk.GET("/", noopHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	wk.ServeHTTP(w, req)

	allow := w.Header().Get("Access-Control-Allow-Headers")
	for _, h := range []string{"X-Octo-Error-Envelope", "X-Octo-Lang", "Accept-Language"} {
		if !strings.Contains(allow, h) {
			t.Errorf("AllowHeaders missing %q: %s", h, allow)
		}
	}

	expose := w.Header().Get("Access-Control-Expose-Headers")
	if expose == "" {
		t.Fatalf("Access-Control-Expose-Headers not set; want Content-Language, Vary")
	}
	for _, h := range []string{"Content-Language", "Vary"} {
		if !strings.Contains(expose, h) {
			t.Errorf("ExposeHeaders missing %q: %s", h, expose)
		}
	}
}

func TestCORSMiddlewareOptionsShortCircuits(t *testing.T) {
	wk := New()
	wk.Use(CORSMiddleware())
	wk.Any("/", noopHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	wk.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want 204", w.Code)
	}
}

// TestCORSMiddlewareAdvertisesScanPollSecret 验证扫码登录的轮询凭据头出现在
// Access-Control-Allow-Headers。
//
// 这个断言看着琐碎，但它挡的是一个静默失效：自定义请求头会让请求变成非简单请求，
// 跨源浏览器先发 OPTIONS 预检；头不在清单里，预检就拒掉**真正的请求**，服务端连
// handler 都进不去。表现是「跨源部署扫码登录完全不可用」，而同源部署一切正常 ——
// 本地和 CI 都测不出来。
func TestCORSMiddlewareAdvertisesScanPollSecret(t *testing.T) {
	wk := New()
	wk.Use(CORSMiddleware())
	wk.GET("/", noopHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	wk.ServeHTTP(w, req)

	allow := w.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(allow, "X-Scan-Poll-Secret") {
		t.Errorf("AllowHeaders missing %q: %s", "X-Scan-Poll-Secret", allow)
	}
}

// TestCORSMiddlewarePreflightAllowsScanPollSecret 走真实的预检形状：带
// Access-Control-Request-Headers 的 OPTIONS，断言 204 且该头被 advertise。
func TestCORSMiddlewarePreflightAllowsScanPollSecret(t *testing.T) {
	wk := New()
	wk.Use(CORSMiddleware())
	wk.GET("/v1/user/loginstatus", noopHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/v1/user/loginstatus", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "X-Scan-Poll-Secret")
	wk.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", w.Code)
	}
	allow := w.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(allow, "X-Scan-Poll-Secret") {
		t.Fatalf("preflight would reject the real request; AllowHeaders = %s", allow)
	}
}

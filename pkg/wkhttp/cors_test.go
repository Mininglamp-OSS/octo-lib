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

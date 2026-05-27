package wkhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeCache 是 cache.Cache 的内存实现，仅用于本包测试。
type fakeCache struct {
	data map[string]string
}

func newFakeCache(seed map[string]string) *fakeCache {
	cp := make(map[string]string, len(seed))
	for k, v := range seed {
		cp[k] = v
	}
	return &fakeCache{data: cp}
}

func (c *fakeCache) Set(k, v string) error                                   { c.data[k] = v; return nil }
func (c *fakeCache) Delete(k string) error                                   { delete(c.data, k); return nil }
func (c *fakeCache) SetAndExpire(k, v string, _ time.Duration) error         { c.data[k] = v; return nil }
func (c *fakeCache) Get(k string) (string, error)                            { return c.data[k], nil }

// TestAuthMiddlewareLegacyFallback 验证三处历史错误分支映射到正确的 i18n key，
// 同时未注入 renderer 时 envelope 与历史完全一致。
func TestAuthMiddlewareLegacyFallback(t *testing.T) {
	cache := newFakeCache(map[string]string{
		"token:goodtoken": "uid1@alice@admin",
		"token:bad":       "not-an-at-separated-value",
	})

	cases := []struct {
		name       string
		token      string
		wantStatus int
		wantMsg    string
	}{
		{"missing", "", http.StatusUnauthorized, "token不能为空，请先登录！"},
		{"not_found", "unknown", http.StatusUnauthorized, "请先登录！"},
		{"invalid_split", "bad", http.StatusUnauthorized, "token有误！"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wk := New()
			mw := wk.AuthMiddleware(cache, "token:")

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.token != "" {
				req.Header.Set("token", tc.token)
			}
			runMiddleware(wk, mw, w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("body not json: %v", err)
			}
			if body["msg"] != tc.wantMsg {
				t.Errorf("msg=%v want=%v", body["msg"], tc.wantMsg)
			}
		})
	}
}

// TestAuthMiddlewareRendererRouted 注入 renderer 后三处错误带各自的 i18n Code。
func TestAuthMiddlewareRendererRouted(t *testing.T) {
	cache := newFakeCache(map[string]string{
		"token:bad": "not-an-at-separated-value",
	})

	cases := []struct {
		name     string
		token    string
		wantCode string
	}{
		{"missing", "", "err.shared.auth.token_missing"},
		{"not_found", "unknown", "err.shared.auth.required"},
		{"invalid_split", "bad", "err.shared.auth.token_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wk := New()
			stub := &stubRenderer{}
			wk.SetErrorRenderer(stub)
			mw := wk.AuthMiddleware(cache, "token:")

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.token != "" {
				req.Header.Set("token", tc.token)
			}
			runMiddleware(wk, mw, w, req)

			if stub.lastSpec().Code != tc.wantCode {
				t.Errorf("Code=%q want=%q", stub.lastSpec().Code, tc.wantCode)
			}
		})
	}
}

// TestAuthMiddlewareSuccessPopulatesContextAndUser 验证成功路径同时写
// c.Set(...) 与 WithUser(ctx, ...)——保证向后兼容 + 新 context 通道可用。
func TestAuthMiddlewareSuccessPopulatesContextAndUser(t *testing.T) {
	cache := newFakeCache(map[string]string{
		"token:ok": "uid1@alice@admin",
	})

	wk := New()
	var seenUID, seenName, seenRole string
	var seenUser UserInfo
	var seenOK bool

	auth := wk.AuthMiddleware(cache, "token:")
	assertHandler := HandlerFunc(func(c *Context) {
		if v, ok := c.Get("uid"); ok {
			seenUID, _ = v.(string)
		}
		if v, ok := c.Get("name"); ok {
			seenName, _ = v.(string)
		}
		if v, ok := c.Get("role"); ok {
			seenRole, _ = v.(string)
		}
		seenUser, seenOK = UserFromCtx(c.Request.Context())
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("token", "ok")
	runChain(wk, []HandlerFunc{auth, assertHandler}, w, req)

	if w.Code != http.StatusOK && w.Code != 0 {
		// httptest.NewRecorder 默认 200；中间件未 abort 时不会显式写 status。
		t.Logf("status code = %d (acceptable if no body written)", w.Code)
	}
	if seenUID != "uid1" || seenName != "alice" || seenRole != "admin" {
		t.Errorf("c.Set values: uid=%q name=%q role=%q", seenUID, seenName, seenRole)
	}
	if !seenOK {
		t.Fatal("UserFromCtx returned ok=false")
	}
	if seenUser.UID != "uid1" || seenUser.Name != "alice" || seenUser.Role != "admin" {
		t.Errorf("UserInfo = %+v", seenUser)
	}
}

// stubTokenParser 让测试控制 Parse 的返回值，验证自定义 parser 优先级与
// sentinel 映射。
type stubTokenParser struct {
	info UserInfo
	err  error
}

func (s stubTokenParser) Parse(_ context.Context, token string) (UserInfo, error) {
	return s.info, s.err
}

// TestAuthMiddlewareUsesInjectedTokenParser 验证 SetTokenParser 之后优先走自定义 parser。
func TestAuthMiddlewareUsesInjectedTokenParser(t *testing.T) {
	wk := New()
	wk.SetTokenParser(stubTokenParser{info: UserInfo{UID: "custom", Name: "n", Role: "r", Language: "en"}})
	mw := wk.AuthMiddleware(newFakeCache(nil), "ignored:")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("token", "any")
	var seenUser UserInfo
	runChain(wk, []HandlerFunc{mw, func(c *Context) {
		seenUser, _ = UserFromCtx(c.Request.Context())
	}}, w, req)

	if seenUser.UID != "custom" || seenUser.Language != "en" {
		t.Errorf("UserInfo = %+v", seenUser)
	}
}

// TestAuthMiddlewareInjectedParserErrorFallsThrough 自定义 parser 返回未识别 error 时，
// 默认归类为 err.shared.auth.required（避免泄露内部错误细节）。
func TestAuthMiddlewareInjectedParserErrorFallsThrough(t *testing.T) {
	wk := New()
	stub := &stubRenderer{}
	wk.SetErrorRenderer(stub)
	wk.SetTokenParser(stubTokenParser{err: errors.New("custom parser failure")})
	mw := wk.AuthMiddleware(newFakeCache(nil), "x:")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("token", "any")
	runMiddleware(wk, mw, w, req)

	if stub.lastSpec().Code != "err.shared.auth.required" {
		t.Errorf("Code=%q want err.shared.auth.required", stub.lastSpec().Code)
	}
}

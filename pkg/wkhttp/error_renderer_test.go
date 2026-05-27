package wkhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

// stubRenderer 把每次渲染的 spec 暂存供断言，并写一个固定 envelope。
// lastSpec 通过 atomic.Pointer 保护，避免并发测试触发 -race 误报。
type stubRenderer struct {
	lastSpecPtr atomic.Pointer[ErrorSpec]
	called      atomic.Int32
}

func (s *stubRenderer) Render(c *Context, spec ErrorSpec) {
	cp := spec
	s.lastSpecPtr.Store(&cp)
	s.called.Add(1)
	c.JSON(spec.TransportStatus, gin.H{
		"code": spec.Code,
		"msg":  spec.DefaultMessage,
	})
}

func (s *stubRenderer) lastSpec() ErrorSpec {
	if p := s.lastSpecPtr.Load(); p != nil {
		return *p
	}
	return ErrorSpec{}
}

// TestDefaultRendererPreservesLegacyEnvelope 验证未注入 renderer 时
// 输出仍是历史 envelope {msg, status}——这是 issue 46 的向后兼容承诺。
func TestDefaultRendererPreservesLegacyEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	ctx := &Context{Context: ginCtx} // wk == nil 触发 fallback 路径
	ctx.RenderError(ErrorSpec{
		Code:            "err.shared.auth.required",
		DefaultMessage:  "请先登录！",
		TransportStatus: http.StatusUnauthorized,
	})

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if body["msg"] != "请先登录！" {
		t.Errorf("msg = %v, want 请先登录！", body["msg"])
	}
	if int(body["status"].(float64)) != http.StatusUnauthorized {
		t.Errorf("status field = %v, want 401", body["status"])
	}
}

// TestSetErrorRendererRouted 验证注入 renderer 后 RenderError 走自定义路径。
func TestSetErrorRendererRouted(t *testing.T) {
	wk := New()
	stub := &stubRenderer{}
	wk.SetErrorRenderer(stub)

	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	ctx := &Context{Context: ginCtx, wk: wk}
	ctx.RenderError(ErrorSpec{
		Code:            "err.test.foo",
		DefaultMessage:  "fallback",
		TransportStatus: http.StatusTeapot,
	})

	if stub.called.Load() != 1 {
		t.Fatalf("renderer not invoked")
	}
	if stub.lastSpec().Code != "err.test.foo" {
		t.Errorf("spec.Code = %q", stub.lastSpec().Code)
	}
	if w.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", w.Code)
	}
}

// TestSetErrorRendererInstanceScoped 验证两个 WKHttp 实例的 renderer 互不污染——
// 这是 issue 46 强调的"实例方法而非全局"的核心保证，回归测试。
func TestSetErrorRendererInstanceScoped(t *testing.T) {
	a, b := New(), New()
	stubA, stubB := &stubRenderer{}, &stubRenderer{}
	a.SetErrorRenderer(stubA)
	b.SetErrorRenderer(stubB)

	wA := httptest.NewRecorder()
	gA, _ := gin.CreateTestContext(wA)
	(&Context{Context: gA, wk: a}).RenderError(ErrorSpec{Code: "x", TransportStatus: 400})

	if stubA.called.Load() != 1 || stubB.called.Load() != 0 {
		t.Fatalf("cross-instance leak: a=%d b=%d", stubA.called.Load(), stubB.called.Load())
	}
}

// TestSetErrorRendererConcurrent 并发设置 renderer 与渲染不应触发数据竞争
// （go test -race 时验证）。
func TestSetErrorRendererConcurrent(t *testing.T) {
	wk := New()
	wk.SetErrorRenderer(&stubRenderer{})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			wk.SetErrorRenderer(&stubRenderer{})
		}()
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			g, _ := gin.CreateTestContext(w)
			(&Context{Context: g, wk: wk}).RenderError(ErrorSpec{Code: "x", TransportStatus: 400})
		}()
	}
	wg.Wait()
}

// TestDefaultRendererZeroTransportStatus 触发 TransportStatus=0 的回退分支——
// 防止调用方遗漏字段时下游误判为 200 成功。
func TestDefaultRendererZeroTransportStatus(t *testing.T) {
	w := httptest.NewRecorder()
	g, _ := gin.CreateTestContext(w)
	(&Context{Context: g}).RenderError(ErrorSpec{Code: "x", DefaultMessage: "m"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 fallback", w.Code)
	}
}

// altRenderer 是另一个 ErrorRenderer 具体类型；与 stubRenderer 同时存在用于验证
// 注入不同具体类型不会触发 atomic 类型不变约束（这是把 atomic.Value 换成 boxed
// atomic.Pointer 的核心动机）。
type altRenderer struct{ tag string }

func (a *altRenderer) Render(c *Context, spec ErrorSpec) {
	c.JSON(spec.TransportStatus, gin.H{"renderer": a.tag, "code": spec.Code})
}

// TestSetErrorRendererSwitchConcreteTypes 验证 SetErrorRenderer 可以反复
// 注入不同具体类型的 renderer 而不会 panic——atomic.Value 会在此处崩溃，
// 这是 P0 修复 atomic.Value → atomic.Pointer[errorRendererBox] 的回归测试。
func TestSetErrorRendererSwitchConcreteTypes(t *testing.T) {
	wk := New()

	// 第一次：stubRenderer
	wk.SetErrorRenderer(&stubRenderer{})
	w1 := httptest.NewRecorder()
	g1, _ := gin.CreateTestContext(w1)
	(&Context{Context: g1, wk: wk}).RenderError(ErrorSpec{Code: "a", TransportStatus: 400})

	// 第二次：altRenderer（不同具体类型）—— atomic.Value 在此处 panic。
	wk.SetErrorRenderer(&altRenderer{tag: "alt"})
	w2 := httptest.NewRecorder()
	g2, _ := gin.CreateTestContext(w2)
	(&Context{Context: g2, wk: wk}).RenderError(ErrorSpec{Code: "b", TransportStatus: 400})

	if !strings.Contains(w2.Body.String(), `"renderer":"alt"`) {
		t.Errorf("second renderer not used: %s", w2.Body.String())
	}

	// 第三次：又换回 stubRenderer
	stub3 := &stubRenderer{}
	wk.SetErrorRenderer(stub3)
	w3 := httptest.NewRecorder()
	g3, _ := gin.CreateTestContext(w3)
	(&Context{Context: g3, wk: wk}).RenderError(ErrorSpec{Code: "c", TransportStatus: 400})
	if stub3.lastSpec().Code != "c" {
		t.Errorf("third renderer not invoked")
	}
}

// TestSetErrorRendererNilResetsToFallback 验证 SetErrorRenderer(nil) 后
// RenderError 回退到 default fallback envelope，且后续可以再次注入。
func TestSetErrorRendererNilResetsToFallback(t *testing.T) {
	wk := New()
	stub := &stubRenderer{}
	wk.SetErrorRenderer(stub)

	// 重置
	wk.SetErrorRenderer(nil)

	w := httptest.NewRecorder()
	g, _ := gin.CreateTestContext(w)
	(&Context{Context: g, wk: wk}).RenderError(ErrorSpec{
		Code:            "err.x",
		DefaultMessage:  "fallback msg",
		TransportStatus: http.StatusUnauthorized,
	})

	if stub.called.Load() != 0 {
		t.Errorf("stub renderer invoked after reset (called=%d)", stub.called.Load())
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status=%d want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "fallback msg") {
		t.Errorf("body=%q, want fallback DefaultMessage", w.Body.String())
	}

	// 再注入：能恢复
	stub2 := &stubRenderer{}
	wk.SetErrorRenderer(stub2)
	w2 := httptest.NewRecorder()
	g2, _ := gin.CreateTestContext(w2)
	(&Context{Context: g2, wk: wk}).RenderError(ErrorSpec{Code: "z", TransportStatus: 400})
	if stub2.called.Load() != 1 {
		t.Errorf("re-injected renderer not invoked")
	}
}

package wkhttp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorSpec 描述一次错误响应的 primitive，供 ErrorRenderer 翻译/渲染。
//
// 设计要点：
//   - Code 是稳定的 i18n key（如 "err.shared.auth.required"），由调用方保证全局唯一。
//   - DefaultMessage 是未注入 renderer 时的兜底文案（与历史硬编码文案一致），
//     保证 octo-lib 单独使用时行为不变。
//   - TransportStatus 是 HTTP 状态码（实际写到响应头）。
//   - SemanticStatus 是业务侧期望的状态码：上游可能因 envelope 协议把 TransportStatus
//     固定为 200，但仍需保留语义 401/403/429——这两个字段允许分离。未来扩展用。
//   - Params 用于翻译模板插值（如 {"retry_seconds": 30}），由 renderer 消费。
//   - Details 携带不需要翻译的结构化字段，会被 renderer 透传到响应体或响应头
//     （例如 rate-limit 的 retry_after）。
//   - Internal=true 时 renderer 应避免把敏感内部细节暴露给客户端。
type ErrorSpec struct {
	Code            string
	DefaultMessage  string
	TransportStatus int
	SemanticStatus  int
	Params          map[string]any
	Details         map[string]any
	Internal        bool
}

// ErrorRenderer 由下游服务实现并通过 WKHttp.SetErrorRenderer 注入。
// 同一个 *WKHttp 实例内只有一个 renderer 生效；不同 WKHttp 实例彼此隔离，
// 因此并发测试中可以为每个实例独立注入而互不污染。
type ErrorRenderer interface {
	Render(c *Context, spec ErrorSpec)
}

// errorRendererBox 包装 ErrorRenderer 以便 atomic.Pointer 持有具体类型。
// 直接存接口需要用 atomic.Value，但 atomic.Value 要求每次 Store 的动态类型
// 一致——注入不同具体实现会 panic，因此采用 boxed 模式。
type errorRendererBox struct {
	r ErrorRenderer
}

// defaultErrorRenderer 是未显式注入 renderer 时的兜底实现，
// 输出与历史 c.AbortWithStatusJSON 形态一致的 envelope：{msg, status}。
// 这样未启用 i18n 的项目升级到本版本后行为完全不变。
type defaultErrorRenderer struct{}

// Render 输出 {msg: spec.DefaultMessage, status: spec.TransportStatus}。
// 若 TransportStatus 为 0（调用方遗漏），回退到 400 以避免下游误判为成功。
func (defaultErrorRenderer) Render(c *Context, spec ErrorSpec) {
	status := spec.TransportStatus
	if status == 0 {
		status = http.StatusBadRequest
	}
	c.JSON(status, gin.H{
		"msg":    spec.DefaultMessage,
		"status": status,
	})
}

// RenderError 通过所属 WKHttp 的 renderer 渲染错误。
// 调用方仍需自行 c.Abort()（与 gin 中间件惯例一致），以确保后续 handler 不再执行。
func (c *Context) RenderError(spec ErrorSpec) {
	r := c.renderer()
	r.Render(c, spec)
}

// renderer 返回当前 Context 所属 WKHttp 的 ErrorRenderer。
// 兜底逻辑：若 Context 未关联 WKHttp（裸调 allocateContext 的单测）或 box 为 nil
// （从未注入 / 注入 nil 重置），返回 defaultErrorRenderer，保证 RenderError 总是可用。
func (c *Context) renderer() ErrorRenderer {
	if c == nil || c.wk == nil {
		return defaultErrorRenderer{}
	}
	box := c.wk.errorRenderer.Load()
	if box == nil || box.r == nil {
		return defaultErrorRenderer{}
	}
	return box.r
}

package wkhttp

import (
	"errors"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/Mininglamp-OSS/octo-lib/pkg/cache"
	"github.com/Mininglamp-OSS/octo-lib/pkg/log"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
)

// UserRole 用户角色
type UserRole string

const (
	// Admin 管理员
	Admin UserRole = "admin"
	// SuperAdmin 超级管理员
	SuperAdmin UserRole = "superAdmin"
)

// WKHttp WKHttp
type WKHttp struct {
	r    *gin.Engine
	pool sync.Pool

	// errorRenderer 存 *errorRendererBox（包装 ErrorRenderer 接口）。
	// 用 atomic.Pointer 而非 atomic.Value 的原因：atomic.Value 要求每次 Store 的
	// 动态类型一致，注入不同具体类型的 renderer 会 panic；测试与多模块场景
	// 下都需要支持任意实现切换，因此与 TokenParser 一致采用 boxed atomic.Pointer。
	errorRenderer atomic.Pointer[errorRendererBox]
	// tokenParser 存 *tokenParserBox（包装 TokenParser 接口）。同上。
	tokenParser atomic.Pointer[tokenParserBox]
}

// New New
func New() *WKHttp {
	l := &WKHttp{
		r:    gin.New(),
		pool: sync.Pool{},
	}
	l.r.Use(gin.Recovery())
	l.pool.New = func() interface{} {
		return allocateContext()
	}
	return l
}

// SetErrorRenderer 注入自定义 ErrorRenderer。线程安全，可在服务启动后任意时刻调用。
// 传 nil 表示恢复 default fallback。
func (l *WKHttp) SetErrorRenderer(r ErrorRenderer) {
	if r == nil {
		l.errorRenderer.Store(nil)
		return
	}
	l.errorRenderer.Store(&errorRendererBox{r: r})
}

func allocateContext() *Context {
	return &Context{Context: nil, lg: log.NewTLog("context")}
}

// Context Context
type Context struct {
	*gin.Context
	lg log.Log
	// wk 反向引用所属 WKHttp，使 c.RenderError 能找到注入的 ErrorRenderer。
	// 由 WKHttpHandler 在每次请求 Get/Put 时设置/清理。
	wk *WKHttp
}

func (c *Context) reset() {
	c.Context = nil
	c.wk = nil
}

// ResponseError ResponseError
//
// Legacy shape {"msg","status"}. New endpoints should respond errors via
// RenderError / the injected ErrorRenderer (envelope.Error wire shape, R1).
func (c *Context) ResponseError(err error) {
	c.JSON(http.StatusBadRequest, gin.H{
		"msg":    err.Error(),
		"status": http.StatusBadRequest,
	})
}

// ResponseErrorf ResponseErrorf
func (c *Context) ResponseErrorf(msg string, err error) {
	if err != nil {
		c.lg.Error(msg, zap.Error(err), zap.String("path", c.FullPath()))
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"msg":    msg,
		"status": http.StatusBadRequest,
	})
}

// ResponseErrorWithStatus ResponseErrorWithStatus
func (c *Context) ResponseErrorWithStatus(err error, status int) {
	c.JSON(status, gin.H{
		"msg":    err.Error(),
		"status": status,
	})
}

// GetPage 获取页参数
func (c *Context) GetPage() (pageIndex int64, pageSize int64) {
	pageIndex, _ = strconv.ParseInt(c.Query("page_index"), 10, 64)
	pageSize, _ = strconv.ParseInt(c.Query("page_size"), 10, 64)
	if pageIndex <= 0 {
		pageIndex = 1
	}
	if pageSize <= 0 {
		pageSize = 15
	}
	return
}

// ResponseOK 返回成功
//
// Legacy shape {"status":200}. New endpoints should prefer ResponseEmpty
// ({"data":{}}, R1); existing clients depend on this shape, do not change it.
func (c *Context) ResponseOK() {
	c.JSON(http.StatusOK, gin.H{
		"status": http.StatusOK,
	})
}

// Response Response
//
// Emits data bare (no envelope). New endpoints should prefer ResponseData /
// ResponseCursor / ResponseOffset (R1 envelope wire shapes).
func (c *Context) Response(data interface{}) {
	c.JSON(http.StatusOK, data)
}

// ResponseWithStatus ResponseWithStatus
func (c *Context) ResponseWithStatus(status int, data interface{}) {
	c.JSON(status, data)
}

// GetLoginUID 获取当前登录的用户uid
func (c *Context) GetLoginUID() string {
	v, ok := c.Get("uid")
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// GetAppID appID
func (c *Context) GetAppID() string {
	return c.GetHeader("appid")
}

// GetLoginName 获取当前登录的用户名字
func (c *Context) GetLoginName() string {
	v, ok := c.Get("name")
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// GetLoginRole 获取当前登录用户的角色
func (c *Context) GetLoginRole() string {
	return c.GetString("role")
}

// GetSpanContext 获取当前请求的span context
func (c *Context) GetSpanContext() opentracing.SpanContext {
	v, ok := c.Get("spanContext")
	if !ok {
		return nil
	}
	sc, _ := v.(opentracing.SpanContext)
	return sc
}

// CheckLoginRole 检查登录角色权限
func (c *Context) CheckLoginRole() error {
	role := c.GetLoginRole()
	if role == "" {
		return errors.New("登录用户角色错误")
	}
	if role != string(Admin) && role != string(SuperAdmin) {
		return errors.New("该用户无权执行此操作")
	}
	return nil
}

// CheckLoginRoleIsSuperAdmin 检查登录用户为超级管理员
func (c *Context) CheckLoginRoleIsSuperAdmin() error {
	role := c.GetLoginRole()
	if role == "" {
		return errors.New("登录用户角色错误")
	}
	if role != string(SuperAdmin) {
		return errors.New("该用户无权执行此操作")
	}
	return nil
}

// HandlerFunc HandlerFunc
type HandlerFunc func(c *Context)

// WKHttpHandler WKHttpHandler
func (l *WKHttp) WKHttpHandler(handlerFunc HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		hc := l.pool.Get().(*Context)
		hc.reset()
		hc.Context = c
		hc.wk = l
		defer l.pool.Put(hc)

		handlerFunc(hc)
	}
}

// Run Run
func (l *WKHttp) Run(addr ...string) error {
	return l.r.Run(addr...)
}

func (l *WKHttp) RunTLS(addr, certFile, keyFile string) error {
	return l.r.RunTLS(addr, certFile, keyFile)
}

// POST POST
func (l *WKHttp) POST(relativePath string, handlers ...HandlerFunc) {
	l.r.POST(relativePath, l.handlersToGinHandleFuncs(handlers)...)
}

// GET GET
func (l *WKHttp) GET(relativePath string, handlers ...HandlerFunc) {
	l.r.GET(relativePath, l.handlersToGinHandleFuncs(handlers)...)
}

// Any Any
func (l *WKHttp) Any(relativePath string, handlers ...HandlerFunc) {
	l.r.Any(relativePath, l.handlersToGinHandleFuncs(handlers)...)
}

// Static Static
func (l *WKHttp) Static(relativePath string, root string) {
	l.r.Static(relativePath, root)
}

// LoadHTMLGlob LoadHTMLGlob
func (l *WKHttp) LoadHTMLGlob(pattern string) {
	l.r.LoadHTMLGlob(pattern)
}

// UseGin UseGin
func (l *WKHttp) UseGin(handlers ...gin.HandlerFunc) {
	l.r.Use(handlers...)
}

// Use Use
func (l *WKHttp) Use(handlers ...HandlerFunc) {
	l.r.Use(l.handlersToGinHandleFuncs(handlers)...)
}

// ServeHTTP ServeHTTP
func (l *WKHttp) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	l.r.ServeHTTP(w, req)
}

// Group Group
func (l *WKHttp) Group(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return newRouterGroup(l.r.Group(relativePath, l.handlersToGinHandleFuncs(handlers)...), l)
}

// HandleContext HandleContext
func (l *WKHttp) HandleContext(c *Context) {
	l.r.HandleContext(c.Context)
}

func (l *WKHttp) handlersToGinHandleFuncs(handlers []HandlerFunc) []gin.HandlerFunc {
	newHandlers := make([]gin.HandlerFunc, 0, len(handlers))
	if handlers != nil {
		for _, handler := range handlers {
			newHandlers = append(newHandlers, l.WKHttpHandler(handler))
		}
	}
	return newHandlers
}

// AuthMiddleware 认证中间件
//
// 三处历史硬编码中文错误改为 c.RenderError：
//   - token 缺失 → err.shared.auth.token_missing
//   - cache 查不到/为空 → err.shared.auth.required
//   - 解析格式错误 → err.shared.auth.token_invalid
//
// 未注入 ErrorRenderer 时由 default fallback 输出 DefaultMessage（与历史完全一致）。
//
// 解析成功后同时执行：
//   - c.Set("uid"/"name"/"role", ...)（向后兼容旧业务代码）
//   - WithUser(c.Request.Context(), info) 写入 context（D20，便于 service 层透传）
//
// 若服务注入了 TokenParser，则用注入的 parser 解析；否则走 legacyTokenParser
// （cache 查 "{prefix}{token}" → "uid@name[@role]" split）。
func (l *WKHttp) AuthMiddleware(cache cache.Cache, tokenPrefix string) HandlerFunc {
	legacy := legacyTokenParser{cache: cache, tokenPrefix: tokenPrefix}

	return func(c *Context) {
		token := c.GetHeader("token")
		if token == "" {
			c.RenderError(ErrorSpec{
				Code:            "err.shared.auth.token_missing",
				DefaultMessage:  "token不能为空，请先登录！",
				TransportStatus: http.StatusUnauthorized,
				SemanticStatus:  http.StatusUnauthorized,
			})
			c.Abort()
			return
		}

		parser := l.getTokenParser()
		if parser == nil {
			parser = legacy
		}
		info, err := parser.Parse(c.Request.Context(), token)
		if err != nil {
			spec := authErrorSpec(err)
			c.RenderError(spec)
			c.Abort()
			return
		}

		c.Set("uid", info.UID)
		c.Set("name", info.Name)
		if info.Role != "" {
			c.Set("role", info.Role)
		}
		c.Request = c.Request.WithContext(WithUser(c.Request.Context(), info))
		c.Next()
	}
}

// authErrorSpec 把 TokenParser 返回的 error 映射到 ErrorSpec。
// 优先用 errors.Is 区分 legacy 三态（missing / invalid / required-fallback）；
// 自定义 parser 若不复用 sentinel，统一回退到 err.shared.auth.required。
func authErrorSpec(err error) ErrorSpec {
	switch {
	case errors.Is(err, ErrTokenMissing):
		return ErrorSpec{
			Code:            "err.shared.auth.token_missing",
			DefaultMessage:  "token不能为空，请先登录！",
			TransportStatus: http.StatusUnauthorized,
			SemanticStatus:  http.StatusUnauthorized,
		}
	case errors.Is(err, ErrTokenInvalid):
		return ErrorSpec{
			Code:            "err.shared.auth.token_invalid",
			DefaultMessage:  "token有误！",
			TransportStatus: http.StatusUnauthorized,
			SemanticStatus:  http.StatusUnauthorized,
		}
	case errors.Is(err, ErrTokenNotFound):
		return ErrorSpec{
			Code:            "err.shared.auth.required",
			DefaultMessage:  "请先登录！",
			TransportStatus: http.StatusUnauthorized,
			SemanticStatus:  http.StatusUnauthorized,
		}
	default:
		// 自定义 parser 未使用 sentinel：默认按"未登录"处理，避免泄露内部错误细节。
		return ErrorSpec{
			Code:            "err.shared.auth.required",
			DefaultMessage:  "请先登录！",
			TransportStatus: http.StatusUnauthorized,
			SemanticStatus:  http.StatusUnauthorized,
		}
	}
}

// GetLoginUID GetLoginUID
func GetLoginUID(token string, tokenPrefix string, cache cache.Cache) string {
	uid, err := cache.Get(tokenPrefix + token)
	if err != nil {
		return ""
	}
	return uid
}

// RouterGroup RouterGroup
type RouterGroup struct {
	*gin.RouterGroup
	L *WKHttp
}

func newRouterGroup(g *gin.RouterGroup, l *WKHttp) *RouterGroup {
	return &RouterGroup{RouterGroup: g, L: l}
}

// POST POST
func (r *RouterGroup) POST(relativePath string, handlers ...HandlerFunc) {
	r.RouterGroup.POST(relativePath, r.L.handlersToGinHandleFuncs(handlers)...)
}

// GET GET
func (r *RouterGroup) GET(relativePath string, handlers ...HandlerFunc) {
	r.RouterGroup.GET(relativePath, r.L.handlersToGinHandleFuncs(handlers)...)
}

// DELETE DELETE
func (r *RouterGroup) DELETE(relativePath string, handlers ...HandlerFunc) {
	r.RouterGroup.DELETE(relativePath, r.L.handlersToGinHandleFuncs(handlers)...)
}

// PUT PUT
func (r *RouterGroup) PUT(relativePath string, handlers ...HandlerFunc) {
	r.RouterGroup.PUT(relativePath, r.L.handlersToGinHandleFuncs(handlers)...)
}

// CORSMiddleware 跨域
//
// AllowHeaders 在历史 header 列表基础上增补 i18n 协议三个头：
//   - X-Octo-Error-Envelope: 客户端声明可解析的错误 envelope 版本
//   - X-Octo-Lang: 客户端显式覆盖语言（优先级高于 Accept-Language）
//   - Accept-Language: 标准协商语言头
//
// ExposeHeaders 是新增字段（历史 CORSMiddleware 完全没有这一行）：
//   - Content-Language: 实际返回的响应语言，浏览器可读
//   - Vary: 让代理 / 浏览器按语言变体缓存
func CORSMiddleware() HandlerFunc {

	return func(c *Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, token, accept, origin, Cache-Control, X-Requested-With, appid, noncestr, sign, timestamp, X-Octo-Error-Envelope, X-Octo-Lang, Accept-Language")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Language, Vary")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT,DELETE,PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

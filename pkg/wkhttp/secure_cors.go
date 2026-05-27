package wkhttp

import (
	"net/http"
	"strings"

	"github.com/Mininglamp-OSS/octo-lib/pkg/log"
	"github.com/gin-gonic/gin"
)

const (
	headerOrigin = "Origin"
	headerACAO   = "Access-Control-Allow-Origin"
	headerACAC   = "Access-Control-Allow-Credentials"
	headerVary   = "Vary"
)

// ParseAllowedOrigins 解析逗号分隔的来源白名单。空项被忽略，前后空白被裁剪。
// 裸 "*" 会被显式丢弃并打 warning——CORS 规范不允许 "*" 与
// Access-Control-Allow-Credentials: true 同时出现，此处也不应接受这种语义。
// 运维若想允许全部来源，应在反代层显式授权或使用精确域名。
func ParseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		if s == "*" {
			log.Warn(`CORS: literal "*" in DM_CORS_ALLOWED_ORIGINS is ignored; use explicit origins or "*.domain" for subdomain matching`)
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// addVaryOrigin 幂等追加 Vary: Origin，兼容单 header 多字段（"Origin, Accept-Encoding"）
// 与多 header 值两种写法，避免与同链路其他中间件产生重复值。
func addVaryOrigin(h http.Header) {
	for _, v := range h.Values(headerVary) {
		for _, field := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(field), headerOrigin) {
				return
			}
		}
	}
	h.Add(headerVary, headerOrigin)
}

// IsOriginAllowed 判断 origin 是否命中白名单。
// 支持精确匹配以及 "*.host" / "scheme://*.host" 的严格子域通配（不匹配裸主机）。
// scheme 与 host 比较均大小写不敏感（遵循 RFC 6454 §4）。
// 注意：不带 scheme 的通配（"*.host"）会同时匹配 http 与 https 来源；
// 要限制为 https，请使用 "https://*.host"。
func IsOriginAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}
	for _, entry := range allowed {
		if entry == "" {
			continue
		}
		if strings.EqualFold(entry, origin) {
			return true
		}
		if matchWildcardOrigin(entry, origin) {
			return true
		}
	}
	return false
}

func matchWildcardOrigin(pattern, origin string) bool {
	patScheme, patHost := splitScheme(pattern)
	oriScheme, oriHost := splitScheme(origin)
	if patScheme != "" && !strings.EqualFold(patScheme, oriScheme) {
		return false
	}
	if !strings.HasPrefix(patHost, "*.") {
		return false
	}
	suffix := strings.ToLower(patHost[1:])
	oriHostLower := strings.ToLower(oriHost)
	return len(oriHostLower) > len(suffix) && strings.HasSuffix(oriHostLower, suffix)
}

func splitScheme(s string) (scheme, host string) {
	if i := strings.Index(s, "://"); i >= 0 {
		return s[:i], s[i+3:]
	}
	return "", s
}

// SecureCORSOverrideMiddleware 返回一个 gin 中间件，用于在上游 CORS 中间件
// 已写入响应头之后，按白名单重写/剥离 Access-Control-Allow-Origin 与
// Access-Control-Allow-Credentials，并追加 Vary: Origin。
//
// **顺序要求**：本中间件必须注册在上游 CORS 中间件**之后**，否则其 Header.Set
// 会被上游覆盖，导致本中间件失效。
//
// 行为：
//   - 请求无 Origin：删除两个 CORS 头，保持同源语义。
//   - Origin 命中白名单：反射该 Origin，Allow-Credentials: true，Vary: Origin。
//   - Origin 未命中：删除两个 CORS 头，仅追加 Vary: Origin。
//
// 注意：预检（OPTIONS）通常被上游中间件提前 Abort，不会进入本中间件。
// 当上游发出的是规范不合法的组合（如 "*" + credentials=true），浏览器本身
// 会拒绝跨域 credentialed 预检，因此攻击路径依然被封堵。预检顺序的彻底
// 修复需要把 CORSMiddleware 与本中间件合并为白名单感知的单一中间件，
// 见 follow-up issue。
//
// 返回类型为 gin.HandlerFunc（而非 wkhttp.HandlerFunc）：本中间件只操作
// 响应头，不需要 c.RenderError 等 *Context 能力，且必须能与上游纯 gin 中间件
// （来自 dmwork-lib 的 server.New 等）混排注册，因此直接使用 gin 原生签名。
func SecureCORSOverrideMiddleware(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get(headerOrigin)
		h := c.Writer.Header()
		if origin == "" {
			h.Del(headerACAO)
			h.Del(headerACAC)
			c.Next()
			return
		}
		addVaryOrigin(h)
		if IsOriginAllowed(origin, allowed) {
			h.Set(headerACAO, origin)
			h.Set(headerACAC, "true")
		} else {
			h.Del(headerACAO)
			h.Del(headerACAC)
		}
		c.Next()
	}
}

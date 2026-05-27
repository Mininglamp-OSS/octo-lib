package wkhttp

import (
	"context"
	"errors"
	"strings"

	"github.com/Mininglamp-OSS/octo-lib/pkg/cache"
)

// TokenParser 由下游服务注入，负责把 token 字符串解析为 UserInfo。
// octo-lib 不感知 token 的具体格式（JWT、自定义 envelope 等），通过此接口解耦。
// 未注入时 AuthMiddleware 回退到 legacyTokenParser（基于 cache + uid@name@role 的旧实现）。
type TokenParser interface {
	Parse(ctx context.Context, token string) (UserInfo, error)
}

// SetTokenParser 注入自定义 token parser；线程安全，可在服务启动后任意时刻调用。
// 同一 *WKHttp 实例最后一次注入生效；nil 表示恢复 legacy 解析。
func (l *WKHttp) SetTokenParser(p TokenParser) {
	if p == nil {
		l.tokenParser.Store(nil)
		return
	}
	l.tokenParser.Store(&tokenParserBox{p: p})
}

// tokenParserBox 包装 TokenParser 以便 atomic.Pointer 持有具体类型。
// 直接存接口会因为接口的双字（type+data）布局触发 atomic.Pointer 的类型不变约束失败。
type tokenParserBox struct {
	p TokenParser
}

// getTokenParser 返回当前注入的 parser，无则返回 nil。
func (l *WKHttp) getTokenParser() TokenParser {
	box := l.tokenParser.Load()
	if box == nil {
		return nil
	}
	return box.p
}

// legacyTokenParser 复现历史 AuthMiddleware 的解析行为：从 cache 取 "{prefix}{token}"
// 得到 "uid@name[@role]"，split 失败返回 errInvalidToken。保留这个 parser 是为了
// 没有显式注入 TokenParser 的下游升级 octo-lib 后无感切换。
type legacyTokenParser struct {
	cache       cache.Cache
	tokenPrefix string
}

// Sentinel errors returned by TokenParser implementations and recognised by
// AuthMiddleware via errors.Is, used to map parse failures to i18n error codes.
// 公开 sentinel 让下游自定义 parser 可以复用同一套语义/翻译键，避免每个 parser
// 自己拟一套字符串。errors.Is(err, ErrTokenInvalid) 是预期的判别方式。
var (
	// ErrTokenMissing —— token header 字面量为空。
	// 映射到 err.shared.auth.token_missing / DefaultMessage "token不能为空，请先登录！"。
	ErrTokenMissing = errors.New("wkhttp: token missing")
	// ErrTokenNotFound —— token 在后端找不到对应会话（cache 未命中、JWT 过期等）。
	// 映射到 err.shared.auth.required / DefaultMessage "请先登录！"。
	ErrTokenNotFound = errors.New("wkhttp: token not found")
	// ErrTokenInvalid —— token 在后端能取到但格式/签名不合法。
	// 映射到 err.shared.auth.token_invalid / DefaultMessage "token有误！"。
	ErrTokenInvalid = errors.New("wkhttp: token invalid")
)

// Parse 实现 TokenParser；ctx 当前未使用（cache.Cache 不接受 context），
// 保留参数是为了未来升级 cache 接口时不破坏 TokenParser 签名。
func (p legacyTokenParser) Parse(_ context.Context, token string) (UserInfo, error) {
	if strings.TrimSpace(token) == "" {
		return UserInfo{}, ErrTokenMissing
	}
	uidAndName, err := p.cache.Get(p.tokenPrefix + token)
	if err != nil || strings.TrimSpace(uidAndName) == "" {
		return UserInfo{}, ErrTokenNotFound
	}
	parts := strings.Split(uidAndName, "@")
	if len(parts) < 2 {
		return UserInfo{}, ErrTokenInvalid
	}
	info := UserInfo{UID: parts[0], Name: parts[1]}
	if len(parts) > 2 {
		info.Role = parts[2]
	}
	return info, nil
}

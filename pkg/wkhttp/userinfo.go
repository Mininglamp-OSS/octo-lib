package wkhttp

import "context"

// UserInfo 是 AuthMiddleware 解析后的登录用户信息 primitive。
// Language 为可选字段，由 TokenParser 在解析 token 时填充（或为空），
// 下游 ErrorRenderer 用它来决定输出语言。
type UserInfo struct {
	UID      string
	Name     string
	Role     string
	Language string
}

type userInfoCtxKey struct{}

// WithUser 把 UserInfo 写入 context；下游中间件/handler 通过 UserFromCtx 读取。
// 这是 c.Set("uid", ...) 的 context.Context 等价物，便于把用户信息透传给
// 不持有 *gin.Context 的下游（如 service 层、goroutine 派发）。
func WithUser(ctx context.Context, info UserInfo) context.Context {
	return context.WithValue(ctx, userInfoCtxKey{}, info)
}

// UserFromCtx 从 context 读取 UserInfo；未注入时返回零值与 false。
func UserFromCtx(ctx context.Context) (UserInfo, bool) {
	if ctx == nil {
		return UserInfo{}, false
	}
	v := ctx.Value(userInfoCtxKey{})
	if v == nil {
		return UserInfo{}, false
	}
	info, ok := v.(UserInfo)
	return info, ok
}

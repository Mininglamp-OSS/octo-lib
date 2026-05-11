package testutil

import (
	"github.com/Mininglamp-OSS/octo-lib/config"
	"github.com/Mininglamp-OSS/octo-lib/module"
	"github.com/Mininglamp-OSS/octo-lib/server"
)

// UID 测试用户ID
var UID = "10000"
var friendUID = "10001"

// Token 测试用户token
var Token = "token122323"

func NewTestContext(cfg *config.Config) *config.Context {
	cfg.Test = true
	ctx := config.NewContext(cfg)
	return ctx
}

// NewTestServer 创建一个测试服务器
func NewTestServer(args ...string) (*server.Server, *config.Context) {
	cfg := config.New()
	cfg.Test = true
	// cfg.TracingOn = true
	// cfg.TracerAddr = "49.235.106.135:6831"
	cfg.DB.MySQLAddr = "root:demo@tcp(127.0.0.1)/test?charset=utf8mb4&parseTime=true"
	cfg.DB.Migration = false
	ctx := config.NewContext(cfg)
	// ctx.Event = event.New(ctx)
	// 先清空旧数据
	err := CleanAllTables(ctx)
	if err != nil {
		panic(err)
	}

	// ctx.Event = event.New(ctx)
	err = ctx.Cache().Set(cfg.Cache.TokenCachePrefix+Token, UID+"@test")
	if err != nil {
		panic(err)
	}

	// 创建server
	s := server.New(ctx)
	// ctx.Server = s
	s.GetRoute().UseGin(ctx.Tracer().GinMiddle())
	ctx.SetHttpRoute(s.GetRoute())
	err = module.Setup(ctx)
	if err != nil {
		panic(err)
	}

	return s, ctx

}

// CleanAllTables 清空所有表
func CleanAllTables(c *config.Context) error {
	var dropSqls []string
	_, err := c.DB().SelectBySql("select  concat('DELETE FROM ','`', table_name,'`') FROM information_schema.tables WHERE table_schema = 'test' and table_name <> 'gorp_migrations'").Load(&dropSqls)
	for _, sql := range dropSqls {
		_, err = c.DB().UpdateBySql(sql).Exec()
		if err != nil {
			return err
		}
	}
	return err
}

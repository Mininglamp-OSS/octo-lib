package config

import (
	"strings"
	"sync"
	"testing"

	"github.com/spf13/viper"
)

func loadCfgWithDB(t *testing.T, yaml string) *Config {
	t.Helper()
	vp := viper.New()
	vp.SetConfigType("yaml")
	if err := vp.ReadConfig(strings.NewReader(yaml)); err != nil {
		t.Fatalf("read config: %v", err)
	}
	cfg := New()
	cfg.ConfigureWithViper(vp)
	return cfg
}

// minimalContext builds a Context wired only for the bits NewRedisCache needs,
// skipping the heavyweight NewContext (event pools, tracer, snowflake).
func minimalContext(cfg *Config) *Context {
	return &Context{
		cfg:       cfg,
		redisOnce: sync.Once{},
	}
}

// Wiring path: RedisTLS=false produces a Conn whose Options carry no TLSConfig.
func TestNewRedisCache_PlainConfig_NoTLSConfig(t *testing.T) {
	cfg := loadCfgWithDB(t, `
db:
  redisAddr: 127.0.0.1:65535
`)
	c := minimalContext(cfg)
	cache := c.NewRedisCache()
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if cache.GetRedisConn().Options().TLSConfig != nil {
		t.Errorf("expected TLSConfig=nil when RedisTLS=false")
	}
}

// Wiring path: RedisTLS=true attaches the TLSConfig produced by BuildTLSConfig.
func TestNewRedisCache_TLSConfig_AttachesTLSConfig(t *testing.T) {
	cfg := loadCfgWithDB(t, `
db:
  redisAddr: 127.0.0.1:65535
  redisTLS: true
  redisTLSInsecureSkipVerify: true
`)
	c := minimalContext(cfg)
	cache := c.NewRedisCache()
	opts := cache.GetRedisConn().Options()
	if opts.TLSConfig == nil {
		t.Fatal("expected non-nil TLSConfig when RedisTLS=true")
	}
	if !opts.TLSConfig.InsecureSkipVerify {
		t.Errorf("expected InsecureSkipVerify=true propagated to TLSConfig")
	}
}

// Wiring path: a bad CA file panics with a wrapped, contextful message.
func TestNewRedisCache_TLSConfig_BadCAFile_Panics(t *testing.T) {
	cfg := loadCfgWithDB(t, `
db:
  redisAddr: 127.0.0.1:65535
  redisTLS: true
  redisTLSCAFile: /definitely/does/not/exist.pem
`)
	c := minimalContext(cfg)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on missing CA file")
		}
		msg, ok := r.(error)
		if !ok {
			t.Fatalf("expected panic value to be error, got %T: %v", r, r)
		}
		if !strings.Contains(msg.Error(), "build redis TLS config") {
			t.Errorf("panic missing wrapping context: %v", msg)
		}
		if !strings.Contains(msg.Error(), "does/not/exist.pem") {
			t.Errorf("panic missing CA file path: %v", msg)
		}
	}()
	_ = c.NewRedisCache()
}

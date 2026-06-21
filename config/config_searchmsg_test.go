package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestSearchMsgDefaults 确认 ESIndex/Search 默认值（off + 端点空 + index 默认）。
func TestSearchMsgDefaults(t *testing.T) {
	cfg := New()
	vp := viper.New()
	cfg.ConfigureWithViper(vp)

	if cfg.Search.ReadBackend != "zinc" {
		t.Errorf("Search.ReadBackend default should be zinc, got %q", cfg.Search.ReadBackend)
	}
	if cfg.ESIndex.On {
		t.Errorf("ESIndex.On default should be false")
	}
	if cfg.ESIndex.Index != "octo-message" {
		t.Errorf("ESIndex.Index default wrong: %q", cfg.ESIndex.Index)
	}
}

// TestSearchMsgFromViperArray YAML 数组形态的 addrs 正确装载。
func TestSearchMsgFromViperArray(t *testing.T) {
	const yaml = `
search:
  readBackend: es
esIndex:
  on: true
  addrs:
    - https://os1:9200
    - https://os2:9200
  username: admin
  password: secret
  index: octo-message
`
	vp := viper.New()
	vp.SetConfigType("yaml")
	if err := vp.ReadConfig(strings.NewReader(yaml)); err != nil {
		t.Fatalf("read config: %v", err)
	}
	cfg := New()
	cfg.ConfigureWithViper(vp)

	if cfg.Search.ReadBackend != "es" {
		t.Errorf("ReadBackend = %q", cfg.Search.ReadBackend)
	}
	if !cfg.ESIndex.On || len(cfg.ESIndex.Addrs) != 2 || cfg.ESIndex.Addrs[0] != "https://os1:9200" {
		t.Errorf("ESIndex addrs array wrong: %+v", cfg.ESIndex.Addrs)
	}
	if cfg.ESIndex.Username != "admin" || cfg.ESIndex.Password != "secret" {
		t.Errorf("ESIndex auth wrong")
	}
}

// TestSearchMsgFromViperCommaScalar 逗号分隔标量形态（env/简写）正确拆分为多元素切片。
func TestSearchMsgFromViperCommaScalar(t *testing.T) {
	vp := viper.New()
	vp.Set("esIndex.addrs", "https://os1:9200,https://os2:9200")

	cfg := New()
	cfg.ConfigureWithViper(vp)

	if len(cfg.ESIndex.Addrs) != 2 || cfg.ESIndex.Addrs[1] != "https://os2:9200" {
		t.Errorf("comma scalar addrs wrong: %v", cfg.ESIndex.Addrs)
	}
}

// TestSearchMsgEmptyKeepsDefault 未配置时保持代码默认（不被覆盖成空切片）。
func TestSearchMsgEmptyKeepsDefault(t *testing.T) {
	vp := viper.New()
	cfg := New()
	cfg.ESIndex.Addrs = []string{"default:9200"}
	cfg.ConfigureWithViper(vp)
	if len(cfg.ESIndex.Addrs) != 1 || cfg.ESIndex.Addrs[0] != "default:9200" {
		t.Errorf("empty config should keep default, got %v", cfg.ESIndex.Addrs)
	}
}

package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestRedisTLSDefaults(t *testing.T) {
	cfg := New()
	vp := viper.New()
	cfg.ConfigureWithViper(vp)

	if cfg.DB.RedisTLS {
		t.Errorf("RedisTLS default should be false, got true")
	}
	if cfg.DB.RedisTLSInsecureSkipVerify {
		t.Errorf("RedisTLSInsecureSkipVerify default should be false, got true")
	}
	if cfg.DB.RedisTLSCAFile != "" {
		t.Errorf("RedisTLSCAFile default should be empty, got %q", cfg.DB.RedisTLSCAFile)
	}
}

func TestRedisTLSFromViper(t *testing.T) {
	const yaml = `
db:
  redisTLS: true
  redisTLSInsecureSkipVerify: true
  redisTLSCAFile: /etc/ssl/certs/ca.pem
`
	vp := viper.New()
	vp.SetConfigType("yaml")
	if err := vp.ReadConfig(strings.NewReader(yaml)); err != nil {
		t.Fatalf("read config: %v", err)
	}

	cfg := New()
	cfg.ConfigureWithViper(vp)

	if !cfg.DB.RedisTLS {
		t.Errorf("expected RedisTLS=true")
	}
	if !cfg.DB.RedisTLSInsecureSkipVerify {
		t.Errorf("expected RedisTLSInsecureSkipVerify=true")
	}
	if got, want := cfg.DB.RedisTLSCAFile, "/etc/ssl/certs/ca.pem"; got != want {
		t.Errorf("RedisTLSCAFile = %q, want %q", got, want)
	}
}

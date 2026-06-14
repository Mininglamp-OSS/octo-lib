package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestSearchMsgDefaults 确认 Kafka/ESIndex/Search 默认值（off + 端点空 + topic/index 默认）。
func TestSearchMsgDefaults(t *testing.T) {
	cfg := New()
	vp := viper.New()
	cfg.ConfigureWithViper(vp)

	if cfg.Search.ReadBackend != "zinc" {
		t.Errorf("Search.ReadBackend default should be zinc, got %q", cfg.Search.ReadBackend)
	}
	if cfg.Kafka.On {
		t.Errorf("Kafka.On default should be false")
	}
	if cfg.Kafka.Topic != "octo.message.v1" || cfg.Kafka.DLQTopic != "octo.message.v1.dlq" {
		t.Errorf("Kafka topic defaults wrong: %q / %q", cfg.Kafka.Topic, cfg.Kafka.DLQTopic)
	}
	if len(cfg.Kafka.Brokers) != 0 {
		t.Errorf("Kafka.Brokers default should be empty, got %v", cfg.Kafka.Brokers)
	}
	if cfg.ESIndex.On {
		t.Errorf("ESIndex.On default should be false")
	}
	if cfg.ESIndex.Index != "octo-message" {
		t.Errorf("ESIndex.Index default wrong: %q", cfg.ESIndex.Index)
	}
}

// TestSearchMsgFromViperArray YAML 数组形态的 brokers/addrs 正确装载。
func TestSearchMsgFromViperArray(t *testing.T) {
	const yaml = `
search:
  readBackend: es
kafka:
  on: true
  brokers:
    - kafka1:9092
    - kafka2:9092
  topic: octo.message.v1
  dlqTopic: octo.message.v1.dlq
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
	if !cfg.Kafka.On || len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[1] != "kafka2:9092" {
		t.Errorf("Kafka brokers array wrong: %+v", cfg.Kafka.Brokers)
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
	vp.Set("kafka.brokers", "kafka1:9092,kafka2:9092 , kafka3:9092")
	vp.Set("esIndex.addrs", "https://os1:9200,https://os2:9200")

	cfg := New()
	cfg.ConfigureWithViper(vp)

	if len(cfg.Kafka.Brokers) != 3 {
		t.Fatalf("comma scalar brokers should split to 3, got %v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.Brokers[0] != "kafka1:9092" || cfg.Kafka.Brokers[2] != "kafka3:9092" {
		t.Errorf("comma scalar brokers trim/split wrong: %v", cfg.Kafka.Brokers)
	}
	if len(cfg.ESIndex.Addrs) != 2 || cfg.ESIndex.Addrs[1] != "https://os2:9200" {
		t.Errorf("comma scalar addrs wrong: %v", cfg.ESIndex.Addrs)
	}
}

// TestSearchMsgEmptyKeepsDefault 未配置时保持代码默认（不被覆盖成空切片）。
func TestSearchMsgEmptyKeepsDefault(t *testing.T) {
	vp := viper.New()
	cfg := New()
	cfg.Kafka.Brokers = []string{"default:9092"}
	cfg.ConfigureWithViper(vp)
	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "default:9092" {
		t.Errorf("empty config should keep default, got %v", cfg.Kafka.Brokers)
	}
}

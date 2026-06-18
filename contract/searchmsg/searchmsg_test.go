package searchmsg

import (
	"encoding/json"
	"testing"
)

// TestMessageJSONRoundTrip 确认契约 JSON 字段名稳定、可往返编解码，且 Content 为
// *string（RawExcluded 时序列化为 null，区别于空串）。同时覆盖 v2 新增的安全字段
// SpaceID / Visibles / MessageSeq（encode→decode 字段不丢）。
func TestMessageJSONRoundTrip(t *testing.T) {
	content := "hello 世界"
	in := Message{
		SchemaVersion: SchemaVersion,
		MessageID:     "123456789012345678",
		ChannelID:     "g_abc",
		ChannelType:   2,
		FromUID:       "u_1",
		SpaceID:       "space_42",
		Visibles:      []string{"u_admin1", "u_admin2"},
		Content:       &content,
		ContentType:   1,
		RawExcluded:   false,
		MsgTimestamp:  1700000000,
		CreatedAt:     1700000001,
		MessageSeq:    18446744073709551615, // max uint64: 全精度往返不截断
		Source:        SourceETLMessageTable,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Message
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.MessageID != in.MessageID || out.ChannelType != in.ChannelType || out.Content == nil || *out.Content != content {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
	if out.Source != SourceETLMessageTable {
		t.Fatalf("source mismatch: %q", out.Source)
	}
	if out.SpaceID != in.SpaceID {
		t.Fatalf("space_id mismatch: got %q want %q", out.SpaceID, in.SpaceID)
	}
	if out.MessageSeq != in.MessageSeq {
		t.Fatalf("message_seq mismatch: got %d want %d", out.MessageSeq, in.MessageSeq)
	}
	if len(out.Visibles) != len(in.Visibles) {
		t.Fatalf("visibles length mismatch: got %v want %v", out.Visibles, in.Visibles)
	}
	for i := range in.Visibles {
		if out.Visibles[i] != in.Visibles[i] {
			t.Fatalf("visibles[%d] mismatch: got %q want %q", i, out.Visibles[i], in.Visibles[i])
		}
	}
}

// TestSchemaVersionIsV2 锁定契约版本：携带安全字段（SpaceID/Visibles/MessageSeq）的
// 契约必须为 v2，indexer LiveContractCarriesSafetyFields()（判 SchemaVersion>=2）据此解封。
func TestSchemaVersionIsV2(t *testing.T) {
	if SchemaVersion != 2 {
		t.Fatalf("SchemaVersion = %d, want 2 (safety-fields contract)", SchemaVersion)
	}
}

// TestSafetyFieldsOmitEmpty 确认三个安全字段在空值时按 omitempty 省略（与 indexer
// esindex.Doc 一致），避免在 producer 未富化时往 ES 写空 keyword/数组扰乱 mapping。
func TestSafetyFieldsOmitEmpty(t *testing.T) {
	in := Message{
		SchemaVersion: SchemaVersion,
		MessageID:     "1",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"space_id", "visibles", "message_seq"} {
		if _, ok := raw[k]; ok {
			t.Fatalf("expected %q omitted when empty, got %s", k, raw[k])
		}
	}
}

// TestSafetyFieldsWireKeys 锁定三个安全字段的 Kafka 线格 JSON key 为 snake_case
// （与契约其余 11 个字段一致；producer json.Marshal(Message) 投 Kafka，consumer
// json.Unmarshal 读回 Message，再由 indexer DocFromMessage 在 Go 字段层映射到
// esindex.Doc 的 camelCase）。这是两条独立线格的边界——Kafka=snake_case、ES=camelCase——
// 故契约侧必须 snake_case，不能照搬 esindex.Doc 的 spaceId/messageSeq。
func TestSafetyFieldsWireKeys(t *testing.T) {
	visibles := []string{"u_admin1"}
	in := Message{
		SchemaVersion: SchemaVersion,
		MessageID:     "1",
		SpaceID:       "space_1",
		Visibles:      visibles,
		MessageSeq:    7,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"space_id", "visibles", "message_seq"} {
		if _, ok := raw[k]; !ok {
			t.Fatalf("expected snake_case wire key %q present, got keys %v", k, keysOf(raw))
		}
	}
	// 反向：不得出现 esindex.Doc 的 camelCase key（防止误把 ES 形态搬进 Kafka 契约）。
	for _, k := range []string{"spaceId", "messageSeq"} {
		if _, ok := raw[k]; ok {
			t.Fatalf("unexpected camelCase key %q in Kafka contract (must be snake_case)", k)
		}
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// TestRawExcludedNullContent 确认 raw_excluded 时 content 序列化为 JSON null。
func TestRawExcludedNullContent(t *testing.T) {
	in := Message{
		SchemaVersion: SchemaVersion,
		MessageID:     "1",
		RawExcluded:   true,
		Content:       nil,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(raw["content"]) != "null" {
		t.Fatalf("expected content null, got %s", raw["content"])
	}
	if string(raw["raw_excluded"]) != "true" {
		t.Fatalf("expected raw_excluded true, got %s", raw["raw_excluded"])
	}
}

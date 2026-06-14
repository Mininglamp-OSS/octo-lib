package searchmsg

import (
	"encoding/json"
	"testing"
)

// TestMessageJSONRoundTrip 确认契约 JSON 字段名稳定、可往返编解码，且 Content 为
// *string（RawExcluded 时序列化为 null，区别于空串）。
func TestMessageJSONRoundTrip(t *testing.T) {
	content := "hello 世界"
	in := Message{
		SchemaVersion: SchemaVersion,
		MessageID:     "123456789012345678",
		ChannelID:     "g_abc",
		ChannelType:   2,
		FromUID:       "u_1",
		Content:       &content,
		ContentType:   1,
		RawExcluded:   false,
		MsgTimestamp:  1700000000,
		CreatedAt:     1700000001,
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

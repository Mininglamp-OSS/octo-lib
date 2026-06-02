package common

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRichText_NewBlocksUnmarshalPreservesOrder(t *testing.T) {
	raw := `{"content":[{"type":"text","text":"看图"},{"type":"image","url":"https://x/a.png","width":100,"height":80},{"type":"text","text":"结束"}]}`
	var p RichTextPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Content) != 3 {
		t.Fatalf("want 3 blocks, got %d", len(p.Content))
	}
	if p.Content[0].Type != RichTextBlockText || p.Content[0].Text != "看图" {
		t.Errorf("block0 wrong: %+v", p.Content[0])
	}
	if p.Content[1].Type != RichTextBlockImage || p.Content[1].URL != "https://x/a.png" || p.Content[1].Width != 100 || p.Content[1].Height != 80 {
		t.Errorf("block1 wrong: %+v", p.Content[1])
	}
	if p.Content[2].Text != "结束" {
		t.Errorf("block2 wrong: %+v", p.Content[2])
	}
}

// 向后兼容：老版本 content 是字符串，不能崩。
func TestRichText_LegacyStringContentBackCompat(t *testing.T) {
	raw := `{"content":"hello world"}`
	var p RichTextPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("legacy unmarshal must not fail: %v", err)
	}
	if len(p.Content) != 1 || p.Content[0].Type != RichTextBlockText || p.Content[0].Text != "hello world" {
		t.Fatalf("legacy content not wrapped into text block: %+v", p.Content)
	}
	if p.Plain != "hello world" {
		t.Errorf("legacy plain backfill = %q, want %q", p.Plain, "hello world")
	}
}

func TestRichText_EmptyAndNullContent(t *testing.T) {
	for _, raw := range []string{`{}`, `{"content":null}`, `{"content":[]}`} {
		var p RichTextPayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatalf("unmarshal %s: %v", raw, err)
		}
		if len(p.Content) != 0 {
			t.Errorf("%s: want 0 blocks, got %d", raw, len(p.Content))
		}
	}
}

func TestBuildRichTextPlain(t *testing.T) {
	content := []RichTextBlock{
		{Type: RichTextBlockText, Text: "前"},
		{Type: RichTextBlockImage, URL: "https://x/a.png"},
		{Type: RichTextBlockText, Text: "后"},
	}
	got := BuildRichTextPlain(content)
	want := "前" + RichTextImagePlaceholder + "后"
	if got != want {
		t.Errorf("plain = %q, want %q", got, want)
	}
}

func TestFillPlain_ServerAuthoritative(t *testing.T) {
	// 端上送了伪造 plain，server FillPlain 必须用 content 重算覆盖。
	p := RichTextPayload{
		Content: []RichTextBlock{
			{Type: RichTextBlockText, Text: "a"},
			{Type: RichTextBlockImage, URL: "https://x/i.png"},
		},
		Plain: "client-forged",
	}
	got := p.FillPlain()
	want := "a" + RichTextImagePlaceholder
	if got != want || p.Plain != want {
		t.Errorf("FillPlain = %q (field %q), want %q", got, p.Plain, want)
	}
}

func TestValidateRichTextBlocks_ImageRules(t *testing.T) {
	if err := ValidateRichTextBlocks([]RichTextBlock{{Type: RichTextBlockImage}}); !errors.Is(err, ErrRichTextImageNoURL) {
		t.Errorf("missing url: got %v, want ErrRichTextImageNoURL", err)
	}
	if err := ValidateRichTextBlocks([]RichTextBlock{{Type: RichTextBlockImage, URL: "data:image/png;base64,AAAA"}}); !errors.Is(err, ErrRichTextImageBase64) {
		t.Errorf("base64 url: got %v, want ErrRichTextImageBase64", err)
	}
	if err := ValidateRichTextBlocks([]RichTextBlock{{Type: "video", Text: "x"}}); !errors.Is(err, ErrRichTextUnknownBlock) {
		t.Errorf("unknown type: got %v, want ErrRichTextUnknownBlock", err)
	}
	if err := ValidateRichTextBlocks([]RichTextBlock{
		{Type: RichTextBlockText, Text: "ok"},
		{Type: RichTextBlockImage, URL: "https://x/a.png", Width: 1, Height: 1},
	}); err != nil {
		t.Errorf("valid blocks rejected: %v", err)
	}
}

func TestValidateRichTextPayload_SizeLimit(t *testing.T) {
	big := make([]byte, RichTextMaxPayloadBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	if _, err := ValidateRichTextPayload(big); !errors.Is(err, ErrRichTextPayloadTooLarge) {
		t.Errorf("oversize: got %v, want ErrRichTextPayloadTooLarge", err)
	}
}

func TestValidateRichTextPayload_LegacyStringDoesNotCrash(t *testing.T) {
	p, err := ValidateRichTextPayload([]byte(`{"content":"legacy"}`))
	if err != nil {
		t.Fatalf("legacy payload validation failed: %v", err)
	}
	if p.Content[0].Text != "legacy" {
		t.Errorf("legacy content lost: %+v", p.Content)
	}
}

func TestGetRichTextDisplayText(t *testing.T) {
	// plain 优先。
	if got := GetRichTextDisplayText([]byte(`{"content":[{"type":"text","text":"x"}],"plain":"已生成"}`)); got != "已生成" {
		t.Errorf("plain priority: got %q", got)
	}
	// plain 空时遍历 content。
	if got := GetRichTextDisplayText([]byte(`{"content":[{"type":"text","text":"hi"},{"type":"image","url":"u"}]}`)); got != "hi"+RichTextImagePlaceholder {
		t.Errorf("derive from content: got %q", got)
	}
	// 空/坏 payload 回退静态名称。
	for _, raw := range []string{``, `not-json`, `{"content":[]}`} {
		if got := GetRichTextDisplayText([]byte(raw)); got != GetDisplayText(RichText.Int()) {
			t.Errorf("fallback for %q: got %q, want %q", raw, got, GetDisplayText(RichText.Int()))
		}
	}
}

// 锁定字段名：序列化必须输出 content / plain，禁止 entities。
func TestRichText_JSONFieldNamesLocked(t *testing.T) {
	p := RichTextPayload{Content: []RichTextBlock{{Type: RichTextBlockText, Text: "x"}}, Plain: "x"}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"content"`) || !strings.Contains(s, `"plain"`) {
		t.Errorf("missing locked field names: %s", s)
	}
	if strings.Contains(s, `"entities"`) || strings.Contains(s, `"offset"`) || strings.Contains(s, `"length"`) {
		t.Errorf("forbidden field names leaked: %s", s)
	}
}

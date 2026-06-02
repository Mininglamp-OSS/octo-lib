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
	if err := ValidateRichTextBlocks([]RichTextBlock{{Type: RichTextBlockImage, Width: 1, Height: 1}}); !errors.Is(err, ErrRichTextImageNoURL) {
		t.Errorf("missing url: got %v, want ErrRichTextImageNoURL", err)
	}
	if err := ValidateRichTextBlocks([]RichTextBlock{{Type: RichTextBlockImage, URL: "data:image/png;base64,AAAA", Width: 1, Height: 1}}); !errors.Is(err, ErrRichTextImageBadScheme) {
		t.Errorf("base64 url: got %v, want ErrRichTextImageBadScheme", err)
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

// 契约 §1.3：text block 的 text 必填且非空（trim 后）。
func TestValidateRichTextBlocks_TextRequired(t *testing.T) {
	cases := []struct {
		name  string
		block RichTextBlock
	}{
		{"missing text", RichTextBlock{Type: RichTextBlockText}},
		{"empty text", RichTextBlock{Type: RichTextBlockText, Text: ""}},
		{"whitespace text", RichTextBlock{Type: RichTextBlockText, Text: "   \t\n"}},
	}
	for _, c := range cases {
		if err := ValidateRichTextBlocks([]RichTextBlock{c.block}); !errors.Is(err, ErrRichTextTextEmpty) {
			t.Errorf("%s: got %v, want ErrRichTextTextEmpty", c.name, err)
		}
	}
	if err := ValidateRichTextBlocks([]RichTextBlock{{Type: RichTextBlockText, Text: "hi"}}); err != nil {
		t.Errorf("non-empty text rejected: %v", err)
	}
}

// 契约 §1.4：image block 的 width/height 必填且 >0。
func TestValidateRichTextBlocks_ImageDimensionsRequired(t *testing.T) {
	cases := []struct {
		name  string
		block RichTextBlock
	}{
		{"no dimensions", RichTextBlock{Type: RichTextBlockImage, URL: "https://x/a.png"}},
		{"zero width", RichTextBlock{Type: RichTextBlockImage, URL: "https://x/a.png", Width: 0, Height: 10}},
		{"zero height", RichTextBlock{Type: RichTextBlockImage, URL: "https://x/a.png", Width: 10, Height: 0}},
		{"negative width", RichTextBlock{Type: RichTextBlockImage, URL: "https://x/a.png", Width: -1, Height: 10}},
		{"negative height", RichTextBlock{Type: RichTextBlockImage, URL: "https://x/a.png", Width: 10, Height: -5}},
	}
	for _, c := range cases {
		if err := ValidateRichTextBlocks([]RichTextBlock{c.block}); !errors.Is(err, ErrRichTextImageNoSize) {
			t.Errorf("%s: got %v, want ErrRichTextImageNoSize", c.name, err)
		}
	}
}

// 契约 §1.4 + §3：image url 走 scheme allowlist（仅 http/https），拒一切其它 scheme。
func TestValidateRichTextBlocks_ImageSchemeAllowlist(t *testing.T) {
	bad := []string{
		"data:image/png;base64,AAAA",
		"DATA:image/png;base64,AAAA", // 大小写绕过
		"  data:text/plain,hello  ",  // 前后空白 + 非 base64 data:
		"javascript:alert(1)",
		"vbscript:msgbox(1)",
		"file:///etc/passwd",
		"ftp://x/a.png",
		"/relative/a.png", // 无 scheme
		"x/a.png",         // 无 scheme
		"http://",         // scheme 对但无 host
		"https://",        // scheme 对但无 host
	}
	for _, u := range bad {
		blk := RichTextBlock{Type: RichTextBlockImage, URL: u, Width: 1, Height: 1}
		if err := ValidateRichTextBlocks([]RichTextBlock{blk}); !errors.Is(err, ErrRichTextImageBadScheme) {
			t.Errorf("scheme %q: got %v, want ErrRichTextImageBadScheme", u, err)
		}
	}
	good := []string{"http://x/a.png", "https://x/a.png", "HTTPS://X/A.PNG"}
	for _, u := range good {
		blk := RichTextBlock{Type: RichTextBlockImage, URL: u, Width: 1, Height: 1}
		if err := ValidateRichTextBlocks([]RichTextBlock{blk}); err != nil {
			t.Errorf("scheme %q rejected: %v", u, err)
		}
	}
}

// 契约 §1.1：顶层 content 必填且非空，拒 缺失 / null / 空数组。
func TestValidateRichTextPayload_ContentRequired(t *testing.T) {
	for _, raw := range []string{`{}`, `{"content":null}`, `{"content":[]}`, `{"plain":"x"}`} {
		if _, err := ValidateRichTextPayload([]byte(raw)); !errors.Is(err, ErrRichTextEmptyContent) {
			t.Errorf("%s: got %v, want ErrRichTextEmptyContent", raw, err)
		}
	}
}

// ValidateRichTextPayload 必须把 ValidateRichTextBlocks 的失败透传（守护 wiring）。
func TestValidateRichTextPayload_RejectsInvalidBlock(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want error
	}{
		{"image no url", `{"content":[{"type":"image","width":1,"height":1}]}`, ErrRichTextImageNoURL},
		{"image no size", `{"content":[{"type":"image","url":"https://x/a.png"}]}`, ErrRichTextImageNoSize},
		{"image data uri", `{"content":[{"type":"image","url":"data:x","width":1,"height":1}]}`, ErrRichTextImageBadScheme},
		{"text empty", `{"content":[{"type":"text","text":""}]}`, ErrRichTextTextEmpty},
		{"unknown type", `{"content":[{"type":"video","text":"x"}]}`, ErrRichTextUnknownBlock},
	}
	for _, c := range cases {
		if _, err := ValidateRichTextPayload([]byte(c.raw)); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

// 大小上限以下的坏 JSON 应返回解析错误（非 nil），不 panic。
func TestValidateRichTextPayload_MalformedJSON(t *testing.T) {
	if _, err := ValidateRichTextPayload([]byte(`{"content":`)); err == nil {
		t.Errorf("malformed JSON under size limit: want parse error, got nil")
	}
}

// FillPlainBounded：入站过校验的 payload，server 注入 plain 后若撑过 1MB 必须被拒。
func TestFillPlainBounded_RejectsOversizeAfterPlain(t *testing.T) {
	// 动态选 n：让入站序列化（plain 为空）恰好 ≤1MB，而 FillPlain 注入大量
	// [图片] 占位符（每块 +8 字节）后撑过 1MB。逐步逼近 cap，避免硬编码块数随
	// 结构微调而失配。
	mk := func(n int) RichTextPayload {
		content := make([]RichTextBlock, n)
		for i := range content {
			content[i] = RichTextBlock{Type: RichTextBlockImage, URL: "https://x/a.png", Width: 1, Height: 1}
		}
		return RichTextPayload{Content: content}
	}
	n := 0
	for {
		next := n + 200
		inbound, err := json.Marshal(mk(next))
		if err != nil {
			t.Fatal(err)
		}
		if len(inbound) > RichTextMaxPayloadBytes {
			break // n 保持为最后一个 ≤cap 的块数
		}
		n = next
	}
	if n < 1 {
		t.Fatalf("could not size payload under cap")
	}
	p := mk(n)
	inbound, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(inbound) > RichTextMaxPayloadBytes {
		t.Fatalf("inbound %d over cap, sizing failed", len(inbound))
	}
	// 入站校验放行。
	if _, err := ValidateRichTextPayload(inbound); err != nil {
		t.Fatalf("inbound payload (%d bytes) unexpectedly rejected: %v", len(inbound), err)
	}
	// server 注入 plain 后复检：应超限。
	if _, err := p.FillPlainBounded(); !errors.Is(err, ErrRichTextPayloadTooLarge) {
		t.Errorf("FillPlainBounded oversize: got %v, want ErrRichTextPayloadTooLarge", err)
	}
}

// FillPlainBounded：正常体积 payload 应通过并回填 plain。
func TestFillPlainBounded_OKForNormalPayload(t *testing.T) {
	p := RichTextPayload{Content: []RichTextBlock{
		{Type: RichTextBlockText, Text: "看图"},
		{Type: RichTextBlockImage, URL: "https://x/a.png", Width: 10, Height: 10},
	}}
	plain, err := p.FillPlainBounded()
	if err != nil {
		t.Fatalf("FillPlainBounded normal payload: %v", err)
	}
	if want := "看图" + RichTextImagePlaceholder; plain != want || p.Plain != want {
		t.Errorf("plain = %q (field %q), want %q", plain, p.Plain, want)
	}
}

// Marshal → Unmarshal 多 block round-trip（自定义 UnmarshalJSON 配默认 Marshal）。
func TestRichText_MarshalUnmarshalRoundTrip(t *testing.T) {
	orig := RichTextPayload{
		Content: []RichTextBlock{
			{Type: RichTextBlockText, Text: "看图"},
			{Type: RichTextBlockImage, URL: "https://x/a.png", Width: 100, Height: 80, Size: 1024, Name: "a.png"},
			{Type: RichTextBlockText, Text: "结束"},
		},
		Plain: "看图[图片]结束",
	}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	var back RichTextPayload
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if len(back.Content) != 3 || back.Plain != orig.Plain {
		t.Fatalf("round-trip mismatch: %+v", back)
	}
	if back.Content[1].URL != "https://x/a.png" || back.Content[1].Width != 100 || back.Content[1].Name != "a.png" {
		t.Errorf("image block round-trip lost fields: %+v", back.Content[1])
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

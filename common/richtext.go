package common

import (
	"encoding/json"
	"errors"
	"strings"
)

// RichText (ContentType=14) 图文混排 blocks payload schema。
//
// 设计基线（两轮跨端审计 YUJ-2740 / YUJ-2745 已定，勿推翻）：
// 图文混排复用已有的 RichText=14（见 common/msg.go），不新增 ContentType。
// 正文以 content 有序数组承载，数组顺序即图文穿插顺序。顶层 plain 为冗余纯
// 文本，由 server 生成，供 search / 推送 / 摘要 / 复制 / 下游 LLM 复用。
//
// 命名约定（锁定，勿改）：字段名 content（block 数组）+ plain（纯文本）。
// 禁止使用 entities + offset/length —— 库内已有多套同名异义 entities
// （robot mention 的 Entitiy{Offset,Length}、其它端的 range 标注等），
// 混用必错，务必避开。

// RichText block 类型常量（显式枚举，二期富文本格式在此扩展）。
const (
	// RichTextBlockText 纯文本块。MVP 锁定 text=纯文本，不渲染 markdown。
	RichTextBlockText = "text"
	// RichTextBlockImage 图片块，只接受 url 引用。
	RichTextBlockImage = "image"
)

// RichTextImagePlaceholder 生成 plain 时 image block 注入的占位符。
const RichTextImagePlaceholder = "[图片]"

// RichTextMaxPayloadBytes RichText payload 序列化后的硬上限（1MB）。
// 图片只允许 url 引用，禁止内嵌 base64，以保证 payload 不超限。
const RichTextMaxPayloadBytes = 1 << 20

// RichText payload 校验错误。
var (
	// ErrRichTextPayloadTooLarge payload 序列化后超过 1MB 上限。
	ErrRichTextPayloadTooLarge = errors.New("richtext payload 超过 1MB 上限")
	// ErrRichTextUnknownBlock content 中出现未知 block 类型。
	ErrRichTextUnknownBlock = errors.New("richtext 未知 block 类型")
	// ErrRichTextImageNoURL image block 缺少 url。
	ErrRichTextImageNoURL = errors.New("richtext image block 缺少 url")
	// ErrRichTextImageBase64 image block 内嵌了 base64（data: URI），仅接受 url 引用。
	ErrRichTextImageBase64 = errors.New("richtext image block 禁止内嵌 base64，只接受 url 引用")
)

// RichTextBlock 是 content 数组中的单个元素。
//   - type=text  用 Text；
//   - type=image 用 URL/Width/Height（Size、Name 可选）。
type RichTextBlock struct {
	// Type 块类型，取值见 RichTextBlockText / RichTextBlockImage。
	Type string `json:"type"`

	// ---- text block ----
	// Text 文本内容（MVP：纯文本，不渲染 markdown）。
	Text string `json:"text,omitempty"`

	// ---- image block ----
	// URL 图片引用地址（禁止 data: base64）。
	URL string `json:"url,omitempty"`
	// Width 图片宽度（像素）。
	Width int `json:"width,omitempty"`
	// Height 图片高度（像素）。
	Height int `json:"height,omitempty"`
	// Size 图片字节大小（可选）。
	Size int `json:"size,omitempty"`
	// Name 图片原始文件名（可选）。
	Name string `json:"name,omitempty"`
}

// RichTextPayload 是 RichText(=14) 消息的 payload。
//   - Content 为有序 block 数组，顺序即图文穿插顺序；
//   - Plain 为冗余纯文本，契约上由 server 生成（见 FillPlain）。
type RichTextPayload struct {
	Content []RichTextBlock `json:"content"`
	Plain   string          `json:"plain"`
}

// UnmarshalJSON 向后兼容老版本 content 为字符串的 RichText payload：
//   - 老 payload {"content":"hello"} 解析为单个 text block，并在 plain 为空时
//     回填为该字符串，保证老的字符串 content 路径不崩；
//   - 新 payload {"content":[...]} 正常解析为 block 数组。
func (p *RichTextPayload) UnmarshalJSON(data []byte) error {
	var raw struct {
		Content json.RawMessage `json:"content"`
		Plain   string          `json:"plain"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Plain = raw.Plain
	p.Content = nil
	if len(raw.Content) == 0 || string(raw.Content) == "null" {
		return nil
	}
	// 优先按新结构（数组）解析。
	var blocks []RichTextBlock
	if err := json.Unmarshal(raw.Content, &blocks); err == nil {
		p.Content = blocks
		return nil
	}
	// 回退：老版本 content 是纯字符串。
	var s string
	if err := json.Unmarshal(raw.Content, &s); err != nil {
		return err
	}
	p.Content = []RichTextBlock{{Type: RichTextBlockText, Text: s}}
	if p.Plain == "" {
		p.Plain = s
	}
	return nil
}

// BuildRichTextPlain 遍历 content blocks 生成纯文本：
//   - text block 取 Text；
//   - image block 注入 RichTextImagePlaceholder；
//   - 数组顺序即拼接顺序。
//
// 契约上 plain 由 server 在入库出口生成，不是端的职责。
func BuildRichTextPlain(content []RichTextBlock) string {
	var b strings.Builder
	for _, blk := range content {
		switch blk.Type {
		case RichTextBlockImage:
			b.WriteString(RichTextImagePlaceholder)
		case RichTextBlockText:
			b.WriteString(blk.Text)
		default:
			// 未知类型（二期扩展前的前向兼容防御）：有 text 则写 text，否则跳过。
			if blk.Text != "" {
				b.WriteString(blk.Text)
			}
		}
	}
	return b.String()
}

// FillPlain 由 server 调用：用 Content 重新生成 plain 并回填（server 权威，
// 不信任端上送的 plain），返回最终 plain。
func (p *RichTextPayload) FillPlain() string {
	p.Plain = BuildRichTextPlain(p.Content)
	return p.Plain
}

// ValidateRichTextBlocks 校验 block 数组：type 必须是已知枚举；image block
// 必带 url 且禁止内嵌 base64（data: URI）。
func ValidateRichTextBlocks(content []RichTextBlock) error {
	for _, blk := range content {
		switch blk.Type {
		case RichTextBlockText:
			// 纯文本，无强约束。
		case RichTextBlockImage:
			if strings.TrimSpace(blk.URL) == "" {
				return ErrRichTextImageNoURL
			}
			if isBase64DataURI(blk.URL) {
				return ErrRichTextImageBase64
			}
		default:
			return ErrRichTextUnknownBlock
		}
	}
	return nil
}

// ValidateRichTextPayload 校验序列化后的 RichText payload：
//  1. 大小不超过 RichTextMaxPayloadBytes；
//  2. 可解析（兼容老的字符串 content，不崩）；
//  3. block 结构合法。
//
// 校验通过后返回解析好的 payload，供调用方直接复用（如 FillPlain）。
func ValidateRichTextPayload(data []byte) (*RichTextPayload, error) {
	if len(data) > RichTextMaxPayloadBytes {
		return nil, ErrRichTextPayloadTooLarge
	}
	var p RichTextPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if err := ValidateRichTextBlocks(p.Content); err != nil {
		return nil, err
	}
	return &p, nil
}

// isBase64DataURI 判断 url 是否为内嵌 data: URI（base64 内联），用于禁内嵌图片。
func isBase64DataURI(url string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(url)), "data:")
}

// GetRichTextDisplayText 给定 RichText(=14) payload，返回用于展示/摘要的纯文本：
//   - 优先用 payload 内的 plain（server 已生成）；
//   - plain 为空则现场遍历 content 生成（兼容老字符串 content）；
//   - 完全无法解析或为空时回退到 GetDisplayText(RichText)（"富文本消息"）。
//
// 这是对既有 GetDisplayText 的补充：GetDisplayText 仅按 type 返回静态名称，
// 不解析 payload；此函数在拿到 payload 时走 plain，且向后兼容。
func GetRichTextDisplayText(payload []byte) string {
	if len(payload) == 0 {
		return GetDisplayText(RichText.Int())
	}
	var p RichTextPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return GetDisplayText(RichText.Int())
	}
	if p.Plain != "" {
		return p.Plain
	}
	if plain := BuildRichTextPlain(p.Content); plain != "" {
		return plain
	}
	return GetDisplayText(RichText.Int())
}

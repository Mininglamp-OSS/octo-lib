package common

import (
	"encoding/json"
	"errors"
	"net/url"
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
	// ErrRichTextEmptyContent 顶层 content 缺失 / null / 空数组（契约必填且非空）。
	ErrRichTextEmptyContent = errors.New("richtext content 必填且不能为空数组")
	// ErrRichTextUnknownBlock content 中出现未知 block 类型。
	ErrRichTextUnknownBlock = errors.New("richtext 未知 block 类型")
	// ErrRichTextTextEmpty text block 的 text 字段缺失/为空（契约必填）。
	ErrRichTextTextEmpty = errors.New("richtext text block 的 text 必填且不能为空")
	// ErrRichTextImageNoURL image block 缺少 url。
	ErrRichTextImageNoURL = errors.New("richtext image block 缺少 url")
	// ErrRichTextImageBadScheme image block 的 url scheme 不在 allowlist（仅 http/https）。
	// 也覆盖内嵌 data: URI —— data:/javascript:/file: 等一律拒绝。
	ErrRichTextImageBadScheme = errors.New("richtext image block 的 url 只接受 http/https scheme")
	// ErrRichTextImageNoSize image block 缺少 width/height 或其值非正（契约必填且 >0）。
	ErrRichTextImageNoSize = errors.New("richtext image block 的 width/height 必填且必须 >0")

	// ErrRichTextImageBase64 已废弃：保留以兼容老调用方的 errors.Is 比较。
	// 新代码统一返回 ErrRichTextImageBadScheme（scheme allowlist 覆盖 data: URI）。
	//
	// Deprecated: 使用 ErrRichTextImageBadScheme。
	ErrRichTextImageBase64 = ErrRichTextImageBadScheme
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
	// URL 图片引用地址（scheme allowlist：仅 http/https；禁 data:/javascript:/file: 等）。
	URL string `json:"url,omitempty"`
	// Width 图片宽度（像素，必填且 >0，供端上占位排版避免抖动）。
	Width int `json:"width,omitempty"`
	// Height 图片高度（像素，必填且 >0）。
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
//
// 注意：FillPlain 只重算 plain，不做大小复检。注入的 [图片] 占位符可能使
// payload 序列化后撑过 1MB（入站过校验 → 出站超限）。server 入库出口应改用
// FillPlainBounded，对回填后的整体 payload 复检大小。
func (p *RichTextPayload) FillPlain() string {
	p.Plain = BuildRichTextPlain(p.Content)
	return p.Plain
}

// FillPlainBounded 是 server 入库出口的权威路径：先 FillPlain 重算 plain，再对
// 回填后的整条 payload 复检序列化大小，避免 server 自己的 plain 注入把 payload
// 撑过 RichTextMaxPayloadBytes（1MB）——入站校验只看原始字节，不含 server 后写的
// plain，故必须在 FillPlain 之后再复检一次。超限返回 ErrRichTextPayloadTooLarge。
func (p *RichTextPayload) FillPlainBounded() (string, error) {
	plain := p.FillPlain()
	out, err := json.Marshal(p)
	if err != nil {
		return plain, err
	}
	if len(out) > RichTextMaxPayloadBytes {
		return plain, ErrRichTextPayloadTooLarge
	}
	return plain, nil
}

// ValidateRichTextBlocks 校验 block 数组（写入/发送端：write-strict）：
//   - 每个 block 的 type 必须是已知枚举（text/image），未知 type 拒绝；
//   - text block：text 必填且非空（契约 §1.3）；
//   - image block：url 必填、scheme 仅允许 http/https（拒 data:/javascript:/file:
//     等一切其它 scheme，契约 §1.4 + §3）、width/height 必填且 >0（契约 §1.4）。
//
// 写入端严格（本函数），展示/解析端宽容（BuildRichTextPlain 对未知 type 降级，
// 见契约 §5.3 Postel）。空 content 由 ValidateRichTextPayload 统一拦截。
func ValidateRichTextBlocks(content []RichTextBlock) error {
	for _, blk := range content {
		switch blk.Type {
		case RichTextBlockText:
			if strings.TrimSpace(blk.Text) == "" {
				return ErrRichTextTextEmpty
			}
		case RichTextBlockImage:
			if strings.TrimSpace(blk.URL) == "" {
				return ErrRichTextImageNoURL
			}
			if !isAllowedImageURLScheme(blk.URL) {
				return ErrRichTextImageBadScheme
			}
			if blk.Width <= 0 || blk.Height <= 0 {
				return ErrRichTextImageNoSize
			}
		default:
			return ErrRichTextUnknownBlock
		}
	}
	return nil
}

// ValidateRichTextPayload 校验序列化后的 RichText payload（写入/发送端 gate）：
//  1. 大小不超过 RichTextMaxPayloadBytes（仅原始入站字节；server 注入 plain 后
//     的复检见 FillPlainBounded）；
//  2. 可解析（兼容老的字符串 content，不崩）；
//  3. 顶层 content 必填且非空（拒 content 缺失 / null / 空数组，契约 §1.1）；
//  4. 每个 block 结构合法（见 ValidateRichTextBlocks）。
//
// 校验通过后返回解析好的 payload，供调用方直接复用（如 FillPlainBounded）。
func ValidateRichTextPayload(data []byte) (*RichTextPayload, error) {
	if len(data) > RichTextMaxPayloadBytes {
		return nil, ErrRichTextPayloadTooLarge
	}
	var p RichTextPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if len(p.Content) == 0 {
		return nil, ErrRichTextEmptyContent
	}
	if err := ValidateRichTextBlocks(p.Content); err != nil {
		return nil, err
	}
	return &p, nil
}

// isAllowedImageURLScheme 判断 image url 的 scheme 是否在 allowlist（仅 http/https）。
// 取代旧的「仅挡 data:」策略：scheme allowlist 同时拦下 data:/javascript:/vbscript:/
// file: 等一切非 http(s) scheme（契约 §1.4 + §3）。无 scheme 的相对地址也拒绝。
func isAllowedImageURLScheme(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}

// GetRichTextDisplayText 给定 RichText(=14) payload，返回用于展示/摘要的纯文本：
//   - 优先用 payload 内的 plain（server 已生成）；
//   - plain 为空（含纯空白）则现场遍历 content 生成（兼容老字符串 content）；
//   - 完全无法解析或为空时回退到 GetDisplayText(RichText)（"富文本消息"）。
//
// ⚠️ 信任边界（契约 §2：端上送的 plain 不可信）：
//   - 本函数走「存储后展示路径」——它信任 plain，前提是 payload 已在入库出口
//     经 (*RichTextPayload).FillPlainBounded() / FillPlain() 用 content 重算覆盖。
//     对已落库、server 权威生成 plain 的 payload，本函数是正确且高效的展示入口。
//   - 「入站校验路径」严禁用本函数判定内容：入站请先 ValidateRichTextPayload，
//     再 FillPlainBounded 重建 plain；不要直接展示端上送的原始 plain（可被伪造）。
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
	if strings.TrimSpace(p.Plain) != "" {
		return p.Plain
	}
	if plain := BuildRichTextPlain(p.Content); plain != "" {
		return plain
	}
	return GetDisplayText(RichText.Int())
}

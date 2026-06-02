# 图文混排 RichText (ContentType=14) blocks 跨端契约

> 状态：Phase 0（协议地基/闸门）。本文档定义 `RichText=14` 的 blocks payload
> schema 与跨端语义约定，五端（iOS / Android / Web / server / bot adapter）后续
> 均 import 本契约实现。本 Phase 只落协议与工具函数，不碰任何业务渲染/发送逻辑。
>
> 来源：两轮跨端审计（YUJ-2740 设计 + YUJ-2745 补漏）。GH-Issue: #56。

## 0. 基线决策（已定，勿推翻）

- **复用 `RichText` ContentType=14**（见 `common/msg.go`），**不新增 type**。
  - `type=20` 已是截屏消息，不可占用。
- 正文以 **`content` 有序数组**承载，**数组顺序 = 图文穿插顺序**。
- 顶层 **`plain`** 为冗余纯文本字段，**由 server 生成**（非端职责），供
  search / 推送 / 摘要 / 复制 / 下游 LLM 复用。
- **命名锁定 `content` / `plain`**。**禁用 `entities` + `offset/length`**：库内已
  有多套同名异义 `entities`（robot mention 的 `Entitiy{Offset,Length}`、其它端的
  range 标注），混用必错，务必避开。

## 1. Payload Schema

RichText(=14) 消息的 `payload`（JSON 对象）：

```jsonc
{
  "content": [                       // 有序数组，顺序即图文穿插顺序
    { "type": "text",  "text": "看这张图：" },
    { "type": "image", "url": "https://cdn/.../a.png", "width": 1080, "height": 720, "size": 81920, "name": "a.png" },
    { "type": "text",  "text": "怎么样？" }
  ],
  "plain": "看这张图：[图片]怎么样？"   // server 生成的冗余纯文本
}
```

### 1.1 顶层字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `content` | `array<block>` | 是 | 有序 block 数组，顺序即展示顺序 |
| `plain` | `string` | 是（语义必填） | 纯文本冗余字段，**由 server 生成**，供 search/推送/摘要/复制/LLM 复用 |

### 1.2 block 公共字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | `string` | 是 | block 类型枚举：`"text"` / `"image"`（显式枚举，二期富文本格式在此扩展） |

### 1.3 `type:"text"` 文本块

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `text` | `string` | 是 | 文本内容。**MVP 锁定：纯文本，不渲染 markdown** |

### 1.4 `type:"image"` 图片块

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | `string` | 是 | 图片引用地址。**只接受 url 引用，禁止内嵌 base64（`data:` URI）** |
| `width` | `int` | 是 | 图片宽度（像素），供端上占位排版避免抖动 |
| `height` | `int` | 是 | 图片高度（像素） |
| `size` | `int` | 否 | 图片字节大小 |
| `name` | `string` | 否 | 原始文件名 |

> Go 类型对应：`common.RichTextPayload` / `common.RichTextBlock`，常量
> `common.RichTextBlockText` / `common.RichTextBlockImage`。

## 2. `plain` 生成规则（server 权威）

`plain` **不是端的职责**，端上送的 `plain` 一律不可信。server 在入库出口调用
`common.RichTextPayload.FillPlain()`（底层 `common.BuildRichTextPlain`）重算并覆盖：

- 遍历 `content`，**按数组顺序**拼接；
- `text` block → 取 `text`；
- `image` block → 注入占位符 **`[图片]`**（`common.RichTextImagePlaceholder`）；
- 结果写回 `plain`。

示例：`[text "看图", image, text "结束"]` → `plain = "看图[图片]结束"`。

## 3. 大小上限与禁内嵌

- payload 序列化后 **硬上限 1MB**（`common.RichTextMaxPayloadBytes`）。
- 图片块 **只接受 url 引用**；内嵌 base64（`data:` URI）被
  `ValidateRichTextBlocks` 拒绝（`ErrRichTextImageBase64`）。这是 1MB 上限能成立
  的前提。

## 4. 向后兼容（老字符串 content 不能崩）

历史上 `RichText` 的 `content` 曾是字符串。本 schema 的 `UnmarshalJSON` 兼容：

- `{"content":"hello"}` → 解析为单个 `text` block（`text="hello"`），且 `plain`
  为空时回填为该字符串；
- `{"content":[...]}` → 正常解析为 block 数组。

老路径解析、校验（`ValidateRichTextPayload`）、展示（`GetRichTextDisplayText`）
均不崩。`GetRichTextDisplayText` 是对既有 `GetDisplayText` 的补充：优先用 `plain`，
其次现场遍历 `content`，最后回退静态名称「富文本消息」。

## 5. 跨端语义约定

### 5.1 part-fail 回滚语义

一条图文混排消息是**原子**的：

- 若任一图片上传/任一 block 准备失败 → **整条 fail，不发送**；
- **不清空草稿（draft）**，用户可重试或编辑后重发；
- 不允许"发出去一半"（部分 block 成功部分失败）。

### 5.2 截断按段（block）规则

需要截断展示/摘要时，**按 block 边界截断，不在 block 内部切断**：

- 累积到长度上限时，丢弃后续整块，不切碎单个 text/image block；
- 摘要/列表预览用 `plain`（其中 image 已是 `[图片]` 占位），按字符上限截断即可。

### 5.3 MVP 锁定项

- `text` block = **纯文本**，**不渲染 markdown**（二期再开富文本格式，届时通过新增
  block `type` 或字段扩展，老端按未知类型前向兼容降级）。

## 6. 明确不做（留后续 Phase）

本 Phase **只做协议 + 工具函数**。以下不在范围：三端渲染 provider、发送端编辑器、
bot adapter 的 enum/case、octo-smart-summary / octo-matter / search 的 `type=14`
分支。

## 7. Go API 速查

| 符号 | 用途 |
|------|------|
| `common.RichTextPayload{Content, Plain}` | payload 结构（含向后兼容 `UnmarshalJSON`） |
| `common.RichTextBlock{Type, Text, URL, Width, Height, Size, Name}` | 单个 block |
| `common.RichTextBlockText` / `common.RichTextBlockImage` | block type 常量 |
| `common.RichTextImagePlaceholder` (`"[图片]"`) | image 占位符 |
| `common.RichTextMaxPayloadBytes` (1MB) | payload 大小上限 |
| `common.BuildRichTextPlain(content)` | 遍历 blocks 生成 plain |
| `(*RichTextPayload).FillPlain()` | server 用 content 重算并回填 plain |
| `common.ValidateRichTextBlocks(content)` | 校验 block 结构 |
| `common.ValidateRichTextPayload(data)` | 校验大小+结构，返回解析结果 |
| `common.GetRichTextDisplayText(payload)` | 取展示文本（plain 优先，向后兼容） |

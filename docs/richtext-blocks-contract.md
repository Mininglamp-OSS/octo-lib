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
| `content` | `array<block>` | 是 | 有序 block 数组，顺序即展示顺序。**必填且非空**：`ValidateRichTextPayload` 拒绝 `content` 缺失 / `null` / 空数组 `[]`（`ErrRichTextEmptyContent`） |
| `plain` | `string` | 是（语义必填） | 纯文本冗余字段，**由 server 生成**，供 search/推送/摘要/复制/LLM 复用 |

### 1.2 block 公共字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | `string` | 是 | block 类型枚举：`"text"` / `"image"`（显式枚举，二期富文本格式在此扩展） |

### 1.3 `type:"text"` 文本块

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `text` | `string` | 是 | 文本内容（trim 后非空）。**MVP 锁定：纯文本，不渲染 markdown**。`ValidateRichTextBlocks` 拒绝缺失/纯空白 `text`（`ErrRichTextTextEmpty`） |

### 1.4 `type:"image"` 图片块

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | `string` | 是 | 图片引用地址。**scheme allowlist：仅 `http`/`https`**；拒 `data:`/`javascript:`/`file:` 等一切其它 scheme（`ErrRichTextImageBadScheme`），亦即禁止内嵌 base64 |
| `width` | `int` | 是（`>0`） | 图片宽度（像素），供端上占位排版避免抖动；缺失/≤0 被拒（`ErrRichTextImageNoSize`） |
| `height` | `int` | 是（`>0`） | 图片高度（像素）；缺失/≤0 被拒（`ErrRichTextImageNoSize`） |
| `size` | `int` | 否 | 图片字节大小 |
| `name` | `string` | 否 | 原始文件名 |

> Go 类型对应：`common.RichTextPayload` / `common.RichTextBlock`，常量
> `common.RichTextBlockText` / `common.RichTextBlockImage`。

## 2. `plain` 生成规则（server 权威）

`plain` **不是端的职责**，端上送的 `plain` 一律不可信。server 在入库出口调用
`common.RichTextPayload.FillPlainBounded()`（底层 `FillPlain` → `common.BuildRichTextPlain`）
重算并覆盖：

- 遍历 `content`，**按数组顺序**拼接；
- `text` block → 取 `text`；
- `image` block → 注入占位符 **`[图片]`**（`common.RichTextImagePlaceholder`）；
- 结果写回 `plain`，并对回填后的整条 payload 复检 1MB 上限（见 §3）。

示例：`[text "看图", image, text "结束"]` → `plain = "看图[图片]结束"`。

### 2.1 信任边界（入站校验路径 vs 存储后展示路径）

端上送的 `plain` 不可信，必须区分两条路径：

- **入站校验路径**：收到端上 payload → `ValidateRichTextPayload`（校验 content/字段）
  → `FillPlainBounded`（用 `content` 重建 `plain` 并复检大小）。**此路径严禁直接信任
  或展示端上送的原始 `plain`**——它可被伪造。
- **存储后展示路径**：payload 已入库、`plain` 已由 server 经 `FillPlainBounded` 权威
  生成。此时 `common.GetRichTextDisplayText` 信任 `plain` 是正确且高效的展示入口
  （函数注释已写明此前提）。

## 3. 大小上限与禁内嵌

- payload 序列化后 **硬上限 1MB**（`common.RichTextMaxPayloadBytes`）。
- 入站校验 `ValidateRichTextPayload` 只校验**原始入站字节**大小；server 注入
  `plain` 后须用 `FillPlainBounded` **复检回填后的整体大小**——`[图片]` 占位符注入
  可能把入站时刚好压线的 payload 撑过 1MB，故出站前必须再检一次（超限返回
  `ErrRichTextPayloadTooLarge`）。
- 图片块 `url` 走 **scheme allowlist：仅 `http`/`https`**；`data:`/`javascript:`/
  `file:` 等一切其它 scheme 被 `ValidateRichTextBlocks` 拒绝
  （`ErrRichTextImageBadScheme`）。禁内嵌 base64（`data:` URI）是其子集，也是 1MB
  上限能成立的前提。

## 4. 向后兼容（老字符串 content 不能崩）

历史上 `RichText` 的 `content` 曾是字符串。本 schema 的 `UnmarshalJSON` 兼容：

- `{"content":"hello"}` → 解析为单个 `text` block（`text="hello"`），且 `plain`
  为空时回填为该字符串；
- `{"content":[...]}` → 正常解析为 block 数组。

老路径解析、校验（`ValidateRichTextPayload`）、展示（`GetRichTextDisplayText`）
均不崩。`GetRichTextDisplayText` 是对既有 `GetDisplayText` 的补充：优先用 `plain`
（仅在已 `FillPlainBounded` 的存储后展示路径可信，见 §2.1），其次现场遍历 `content`，
最后回退静态名称「富文本消息」。

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

### 5.4 未知 block type 的 Postel 策略（write-strict / read-lenient）

本契约对未知 block `type` 采用 **Postel 原则**——发送时严格，解析时宽容：

- **write-strict（写入/发送端）**：`ValidateRichTextBlocks` / `ValidateRichTextPayload`
  对未知 `type` **硬拒**（`ErrRichTextUnknownBlock`）。MVP 发送端只准 `text` / `image`，
  不允许写出未登记的 block type。
- **read-lenient（解析/展示端）**：`BuildRichTextPlain` / `GetRichTextDisplayText`
  遇未知 block **不崩**——有 `text` 字段则降级取其文本，否则跳过该块，继续渲染其余
  block。这保证二期新增 block type 后，老端解析新消息不会整条失败。

由此，二期扩展新 block type 时：升级发送端的 write 校验放行新 type，老解析端无需改动
即可前向兼容降级。**注意**：因校验先于展示，老 server 的入站 gate 会先于宽容展示路径
拒掉新 type 的**写入**——这是有意为之（gate 防止未登记内容入库），新 type 的落地需要
先升级 server 端校验白名单。

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
| `common.BuildRichTextPlain(content)` | 遍历 blocks 生成 plain（read-lenient：未知 type 降级） |
| `(*RichTextPayload).FillPlain()` | server 用 content 重算并回填 plain（不复检大小） |
| `(*RichTextPayload).FillPlainBounded()` | 入库出口权威路径：FillPlain + 回填后 1MB 复检 |
| `common.ValidateRichTextBlocks(content)` | 校验 block 结构（write-strict：必填字段 + scheme allowlist） |
| `common.ValidateRichTextPayload(data)` | 校验大小+content 非空+结构，返回解析结果 |
| `common.GetRichTextDisplayText(payload)` | 取展示文本（plain 优先，仅存储后路径可信，见 §2.1） |

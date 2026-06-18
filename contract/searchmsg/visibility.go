package searchmsg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrVisibilityFailClosed 表示一条**非加密**消息的可见性无法被可信解析，调用方必须
// fail-closed（落 DLQ，绝不进正文 topic）。判 errors.Is(err, ErrVisibilityFailClosed)
// 可与「真序列化/IO 错误」区分，但当前所有 fail-closed 返回都包它，调用方只需判 err!=nil。
var ErrVisibilityFailClosed = errors.New("searchmsg: visibility fail-closed")

// ExtractVisibility 从一条消息的**原始 payload 字节**解析 reader 鉴权所需的可见性字段
// （SpaceID / Visibles）——这是 octo-im 消息检索管线 fail-closed 可见性口径的**单一真源**。
//
// 三方共用（强制 (a) 抽 octo-lib，禁 (b) 各仓重实现，防 #1124 口径分叉）：
//   - octo-server searchetl producer（实时 + backfill 富化时填 searchmsg.Message.SpaceID/Visibles）
//   - octo-search-indexer backfill（docFromRow 富化 esindex.Doc.SpaceID/Visibles）
//   - 未来 reader 侧若需从 payload 复核可见性
//
// 仅对**非加密**消息调用：Signal 加密 DM 的 payload 是密文，调用方应在调用前判加密并直接
// 走 raw_excluded（SpaceID/Visibles 留空 = reader fail-closed，安全方向），绝不把密文当损坏
// JSON 喂进来误判。
//
// 🔴 fail-closed 返回值契约（reader visibility.go 对空 Visibles 是 **fail-OPEN**：普通成员
// 能搜出「仅指定成员可见」的群定向系统消息，正是 #1124 泄漏）：
//
//	① payload 顶层不是 JSON 对象（损坏 / 截断 / 数组 / 标量）
//	    → err != nil。可见性整体不可信，调用方 fail-closed（DLQ）。
//	② payload 是对象，但**不含** visibles 键
//	    → spaceID（容忍解析）, nil, nil。这是「广播给频道全员」的正常消息（普通群聊正文、
//	      GroupCreate/GroupUpdate 等无定向系统消息）——reader 无 gate 即放行属预期，**安全**。
//	③ payload 含 visibles 键，但解析不出 **≥1 个有效字符串 UID**
//	    （visibles 为 null / 非数组 / 空数组 [] / 数组里全是非字符串或空串元素）
//	    → err != nil。**关键**：visibles 键的「存在」即表示「本消息本应受白名单约束」（octo-lib
//	      config/msg_group.go 仅在 GroupMemberBeRemove/Invite/Exit 等定向系统消息上写 visibles=
//	      subscribers，且发送前已保证 subscribers 非空）。若键在却解析为空，说明白名单已损坏/漂移
//	      → 必须 fail-closed（DLQ），绝不写空 Visibles 让 reader fail-OPEN。这正是 ReviewBot
//	      YUJ-4953 钉死的「valid-but-empty visibles 也要拦」口径，比旧 indexer extractVisibility
//	      （把空数组当广播放行）更严。
//	④ payload 含合法 visibles（≥1 个有效字符串 UID）
//	    → spaceID, visibles, nil。
//
// space_id 的类型**容忍**：仅当 JSON 字符串时取值，其余类型（数字 / 对象 / null / 上游漂移）
// 退化为空 spaceID（reader 对 p2p 空 spaceID 走 fail-closed，安全方向）——绝不让 space_id 的
// 怪异类型炸掉整条 payload 的解析、连累合法 visibles 被清空（这是 V3b fail-OPEN 的历史根因）。
func ExtractVisibility(payload []byte) (spaceID string, visibles []string, err error) {
	// 先把 payload 解成顶层对象（容忍每个字段各自的 JSON 类型）。顶层不是对象才是真正
	// 「可见性不可信」→ fail-closed。单个字段的类型怪异不在此判死。
	var top map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(payload))
	if derr := dec.Decode(&top); derr != nil {
		return "", nil, fmt.Errorf("%w: payload not a JSON object: %v", ErrVisibilityFailClosed, derr)
	}
	// 顶层 `null` 会 Decode 成 nil map 而**不**报错——它不是合法的可见性对象，必须 fail-closed
	// （否则落进「无 visibles 键」分支被当广播放行 = fail-OPEN）。
	if top == nil {
		return "", nil, fmt.Errorf("%w: payload is JSON null, not an object", ErrVisibilityFailClosed)
	}
	// 拒绝尾部多余字节（如 `{...} garbage`、`{...}]`、`{...}}`）：合法 payload 必须**恰好**是
	// 一个 JSON 对象，其后只能是空白 + EOF。不能用 dec.More()——它把 `]`/`}` 当「无下一元素」
	// 返回 false，会放过 `{...}]` 这类损坏 payload（落进「无 visibles 键」分支被当广播 = fail-OPEN）。
	// 改为再 Decode 一次并要求返回 io.EOF：任何可再解出的值或非 EOF 错误都说明有尾部残留 → fail-closed。
	var trailing json.RawMessage
	if derr := dec.Decode(&trailing); !errors.Is(derr, io.EOF) {
		return "", nil, fmt.Errorf("%w: payload has trailing data after JSON object", ErrVisibilityFailClosed)
	}

	// space_id：容忍类型——仅 JSON 字符串取值，其余类型留空（不报错、不连累 visibles）。
	if raw, ok := top["space_id"]; ok {
		var s string
		if uerr := json.Unmarshal(raw, &s); uerr == nil {
			spaceID = s
		}
		// 非字符串 space_id（数字 / 对象 / null）→ spaceID 留空（reader p2p fail-closed）。
	}

	// visibles：键**不存在** = 广播消息（无 gate 意图），放行进正文流（安全：本就发给全员）。
	rawVis, ok := top["visibles"]
	if !ok {
		return spaceID, nil, nil
	}

	// 键存在即表示「本应受白名单约束」。从这里起，任何「解析不出 ≥1 个有效 UID」都 fail-closed。
	// null：键在却无值 = 白名单损坏/漂移 → fail-closed（绝不当广播放行）。
	if string(bytes.TrimSpace(rawVis)) == "null" {
		return "", nil, fmt.Errorf("%w: visibles present but null", ErrVisibilityFailClosed)
	}
	var elems []json.RawMessage
	if uerr := json.Unmarshal(rawVis, &elems); uerr != nil {
		// 非数组（对象 / 标量 / 字符串）= 白名单结构损坏 → fail-closed。
		return "", nil, fmt.Errorf("%w: visibles is not a JSON array: %v", ErrVisibilityFailClosed, uerr)
	}
	out := make([]string, 0, len(elems))
	for _, raw := range elems {
		var s string
		if uerr := json.Unmarshal(raw, &s); uerr != nil {
			// 与 octo-server visiblesAllows 口径一致：非字符串元素跳过（不当作有效约束）。
			continue
		}
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		// visibles 键在、却解析不出任何有效 UID（空数组 [] / 全非字符串 / 全空串）
		// = valid-but-empty 白名单损坏 → fail-closed（ReviewBot YUJ-4953 钉死口径）。
		return "", nil, fmt.Errorf("%w: visibles present but resolves to zero valid UIDs", ErrVisibilityFailClosed)
	}
	return spaceID, out, nil
}

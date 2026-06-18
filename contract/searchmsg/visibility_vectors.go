package searchmsg

// 本文件是**非测试**源码（不带 _test.go 后缀），故可被任意外部仓 import。它把
// ExtractVisibility 的 fail-closed 测试向量收敛为单一真源，供三方共用，锁口径：
//   - octo-server  modules/searchetl 的 producer fail-closed 单测
//   - octo-search-indexer backfill 富化路径单测
// 两侧跑**同一组**向量（验收门 (ii)：同口径锁），防 #1124 在不同仓重新分叉。

// VisibilityVector 是一条 ExtractVisibility 的期望行为向量。
type VisibilityVector struct {
	// Name 用例名（断言失败时定位）。
	Name string
	// Payload 原始消息 payload 字节（非加密）。
	Payload []byte
	// WantErr 期望 fail-closed（true → 调用方必须落 DLQ，绝不进正文 topic）。
	WantErr bool
	// WantSpaceID / WantVisibles 仅在 WantErr=false 时校验。WantVisibles=nil 表示
	// 「广播消息，无白名单」（reader 无 gate 放行属预期安全行为）。
	WantSpaceID  string
	WantVisibles []string
}

// FailClosedVisibilityVectors 返回锁口径的共享测试向量集。
//
// 安全核心（ReviewBot YUJ-4953 裁决逐字钉死，不可弱化）：
//   - 非加密群消息 visibles **解析失败** → fail-closed（DLQ）。
//   - 非加密群消息 visibles **valid-but-empty**（键在但空数组 / null / 全非字符串）
//     → 同样 fail-closed（DLQ）。population 校验主落点在此，空数组也要拦。
//   - visibles 键**缺失** = 广播消息 → 放行（visibles=nil），这是正常群聊正文/无定向系统
//     消息，reader 无 gate 放行属预期；若把它也拦死会把全部正常消息灌进 DLQ。
//   - space_id 非字符串类型不连累 visibles（V3b fail-OPEN 根因隔离）。
func FailClosedVisibilityVectors() []VisibilityVector {
	return []VisibilityVector{
		// ---- 放行类（visibles 键缺失 = 广播；或含合法白名单）----
		{
			Name:         "broadcast/no_visibles_key",
			Payload:      []byte(`{"type":1,"content":"hello group"}`),
			WantErr:      false,
			WantVisibles: nil,
		},
		{
			Name:         "broadcast/no_visibles_with_space_id",
			Payload:      []byte(`{"type":1,"content":"hi","space_id":"space_42"}`),
			WantErr:      false,
			WantSpaceID:  "space_42",
			WantVisibles: nil,
		},
		{
			Name:         "valid/visibles_single_uid",
			Payload:      []byte(`{"type":99,"content":"you were removed","visibles":["u_alice"]}`),
			WantErr:      false,
			WantVisibles: []string{"u_alice"},
		},
		{
			Name:         "valid/visibles_multi_uid_with_space",
			Payload:      []byte(`{"content":"x","visibles":["u_a","u_b"],"space_id":"s1"}`),
			WantErr:      false,
			WantSpaceID:  "s1",
			WantVisibles: []string{"u_a", "u_b"},
		},
		{
			// space_id 非字符串（数字）必须退化为空 spaceID，且**绝不**连累合法 visibles。
			Name:         "tolerant/space_id_number_does_not_drop_visibles",
			Payload:      []byte(`{"content":"x","space_id":123,"visibles":["u_a","u_b"]}`),
			WantErr:      false,
			WantSpaceID:  "",
			WantVisibles: []string{"u_a", "u_b"},
		},
		{
			// 数组里混入非字符串元素：跳过非法元素，仍保留有效 UID（与 visiblesAllows 一致）。
			Name:         "valid/visibles_mixed_skips_non_string",
			Payload:      []byte(`{"content":"x","visibles":["u_a",123,null,"u_b"]}`),
			WantErr:      false,
			WantVisibles: []string{"u_a", "u_b"},
		},

		// ---- fail-closed 类：解析失败（unparseable）----
		{
			Name:    "failclosed/payload_not_json",
			Payload: []byte(`{not valid json`),
			WantErr: true,
		},
		{
			Name:    "failclosed/payload_top_level_array",
			Payload: []byte(`["a","b"]`),
			WantErr: true,
		},
		{
			Name:    "failclosed/payload_top_level_scalar",
			Payload: []byte(`"just a string"`),
			WantErr: true,
		},
		{
			// 顶层 JSON null：Decode 成 nil map 不报错，但不是合法可见性对象 → fail-closed。
			Name:    "failclosed/payload_json_null",
			Payload: []byte(`null`),
			WantErr: true,
		},
		{
			// 尾部多余字节：Decode 只吃第一个值，残留 = payload 不是恰好一个对象 → fail-closed。
			Name:    "failclosed/payload_trailing_data",
			Payload: []byte(`{"content":"x"} garbage`),
			WantErr: true,
		},
		{
			// 尾部多余 `]`：dec.More() 会误判为「无更多」，必须靠二次 Decode 要求 EOF 才能拦住。
			Name:    "failclosed/payload_trailing_bracket",
			Payload: []byte(`{"content":"x"}]`),
			WantErr: true,
		},
		{
			// 尾部多余 `}`：同上，损坏 payload 不可信 → fail-closed。
			Name:    "failclosed/payload_trailing_brace",
			Payload: []byte(`{"content":"x"}}`),
			WantErr: true,
		},
		{
			Name:    "failclosed/visibles_object_not_array",
			Payload: []byte(`{"content":"x","visibles":{"u_a":true}}`),
			WantErr: true,
		},
		{
			Name:    "failclosed/visibles_string_not_array",
			Payload: []byte(`{"content":"x","visibles":"u_a"}`),
			WantErr: true,
		},

		// ---- fail-closed 类：valid-but-empty（ReviewBot 特别强调，空数组也要拦）----
		{
			Name:    "failclosed/visibles_empty_array",
			Payload: []byte(`{"content":"x","visibles":[]}`),
			WantErr: true,
		},
		{
			Name:    "failclosed/visibles_null",
			Payload: []byte(`{"content":"x","visibles":null}`),
			WantErr: true,
		},
		{
			Name:    "failclosed/visibles_all_non_string",
			Payload: []byte(`{"content":"x","visibles":[123,456,null]}`),
			WantErr: true,
		},
		{
			Name:    "failclosed/visibles_all_empty_string",
			Payload: []byte(`{"content":"x","visibles":["",""]}`),
			WantErr: true,
		},
	}
}

package searchmsg

import (
	"errors"
	"reflect"
	"testing"
)

// TestExtractVisibility_SharedVectors 跑共享 fail-closed 向量集，锁口径单一真源。
// octo-search-indexer 的 producer 与 backfill 各自 import 同一组向量跑同样断言，
// 防 #1124 在不同仓重新分叉（验收门 (ii)）。
func TestExtractVisibility_SharedVectors(t *testing.T) {
	for _, v := range FailClosedVisibilityVectors() {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			spaceID, visibles, err := ExtractVisibility(v.Payload)
			if v.WantErr {
				if err == nil {
					t.Fatalf("%s: want fail-closed err, got spaceID=%q visibles=%v", v.Name, spaceID, visibles)
				}
				if !errors.Is(err, ErrVisibilityFailClosed) {
					t.Fatalf("%s: err must wrap ErrVisibilityFailClosed, got %v", v.Name, err)
				}
				// fail-closed 时绝不返回任何可见性值（防调用方误用）。
				if spaceID != "" || visibles != nil {
					t.Fatalf("%s: fail-closed must return empty values, got spaceID=%q visibles=%v", v.Name, spaceID, visibles)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: want ok, got err %v", v.Name, err)
			}
			if spaceID != v.WantSpaceID {
				t.Fatalf("%s: spaceID=%q want %q", v.Name, spaceID, v.WantSpaceID)
			}
			if !reflect.DeepEqual(visibles, v.WantVisibles) {
				t.Fatalf("%s: visibles=%v want %v", v.Name, visibles, v.WantVisibles)
			}
		})
	}
}

// TestExtractVisibility_EmptyVisiblesFailClosed 单独钉死 ReviewBot 最强调的口径：
// 非加密群消息 visibles **valid-but-empty**（键在、空数组）必须 fail-closed，绝不放空 visibles。
func TestExtractVisibility_EmptyVisiblesFailClosed(t *testing.T) {
	_, _, err := ExtractVisibility([]byte(`{"content":"x","visibles":[]}`))
	if err == nil {
		t.Fatal("valid-but-empty visibles MUST fail-closed (else reader fail-OPEN, #1124)")
	}
}

// TestExtractVisibility_NoVisiblesIsBroadcast 钉死：无 visibles 键 = 广播，放行不报错。
// 否则全部正常群聊正文会被灌进 DLQ。
func TestExtractVisibility_NoVisiblesIsBroadcast(t *testing.T) {
	spaceID, visibles, err := ExtractVisibility([]byte(`{"type":1,"content":"normal chat"}`))
	if err != nil {
		t.Fatalf("broadcast (no visibles key) must pass, got %v", err)
	}
	if spaceID != "" || visibles != nil {
		t.Fatalf("broadcast must yield empty spaceID + nil visibles, got %q %v", spaceID, visibles)
	}
}

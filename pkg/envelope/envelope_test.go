package envelope

import (
	"encoding/json"
	"testing"
)

// marshal returns the compact JSON for v, failing the test on error.
func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestDataShape(t *testing.T) {
	type matter struct {
		MatterID string `json:"matter_id"`
	}
	got := marshal(t, Data[matter]{Data: matter{MatterID: "m1"}})
	want := `{"data":{"matter_id":"m1"}}`
	if got != want {
		t.Errorf("Data[T] = %s, want %s", got, want)
	}
}

func TestDataEmptyRespShape(t *testing.T) {
	got := marshal(t, Data[EmptyResp]{})
	want := `{"data":{}}`
	if got != want {
		t.Errorf("Data[EmptyResp] = %s, want %s", got, want)
	}
}

func TestCursorListShape(t *testing.T) {
	got := marshal(t, CursorList[string]{
		Data:       []string{"a", "b"},
		Pagination: CursorPagination{HasMore: true, NextCursor: "tok"},
	})
	want := `{"data":["a","b"],"pagination":{"has_more":true,"next_cursor":"tok"}}`
	if got != want {
		t.Errorf("CursorList[T] = %s, want %s", got, want)
	}
}

func TestCursorListLastPageOmitsNextCursor(t *testing.T) {
	got := marshal(t, CursorList[string]{
		Data:       []string{"a"},
		Pagination: CursorPagination{HasMore: false},
	})
	// has_more must stay explicit even when false (R5: clients must not
	// infer the end from an empty array); next_cursor is omitted.
	want := `{"data":["a"],"pagination":{"has_more":false}}`
	if got != want {
		t.Errorf("CursorList last page = %s, want %s", got, want)
	}
}

func TestOffsetListShape(t *testing.T) {
	got := marshal(t, OffsetList[string]{
		Data:       []string{"a"},
		Pagination: OffsetPagination{Total: 42, Page: 2, PageSize: 20},
	})
	want := `{"data":["a"],"pagination":{"total":42,"page":2,"page_size":20}}`
	if got != want {
		t.Errorf("OffsetList[T] = %s, want %s", got, want)
	}
}

func TestErrorShape(t *testing.T) {
	got := marshal(t, Error{Error: ErrorBody{
		Code:    "NOT_FOUND",
		Message: "Matter not found",
		Details: map[string]any{"resource": "matter"},
		Hint:    "Verify the matter_id and try again.",
	}})
	want := `{"error":{"code":"NOT_FOUND","message":"Matter not found","details":{"resource":"matter"},"hint":"Verify the matter_id and try again."}}`
	if got != want {
		t.Errorf("Error = %s, want %s", got, want)
	}
}

func TestErrorShapeOmitsEmptyDetailsAndHint(t *testing.T) {
	got := marshal(t, Error{Error: ErrorBody{Code: "INTERNAL_ERROR", Message: "internal error"}})
	want := `{"error":{"code":"INTERNAL_ERROR","message":"internal error"}}`
	if got != want {
		t.Errorf("Error minimal = %s, want %s", got, want)
	}
}

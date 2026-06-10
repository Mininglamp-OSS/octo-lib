package wkhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestContext returns a Context wired to a recorder, with an optional
// request query string (e.g. "page=2&page_size=50").
func newTestContext(t *testing.T, rawQuery string) (*Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
	return &Context{Context: ginCtx}, w
}

func assertResponse(t *testing.T, w *httptest.ResponseRecorder, wantStatus int, wantBody string) {
	t.Helper()
	if w.Code != wantStatus {
		t.Errorf("status = %d, want %d", w.Code, wantStatus)
	}
	if got := w.Body.String(); got != wantBody {
		t.Errorf("body = %s, want %s", got, wantBody)
	}
}

type testItem struct {
	ID string `json:"id"`
}

func TestResponseData(t *testing.T) {
	ctx, w := newTestContext(t, "")
	ctx.ResponseData(testItem{ID: "x"})
	assertResponse(t, w, http.StatusOK, `{"data":{"id":"x"}}`)
}

func TestResponseCreated(t *testing.T) {
	ctx, w := newTestContext(t, "")
	ctx.ResponseCreated(testItem{ID: "x"})
	assertResponse(t, w, http.StatusCreated, `{"data":{"id":"x"}}`)
}

func TestResponseEmpty(t *testing.T) {
	ctx, w := newTestContext(t, "")
	ctx.ResponseEmpty()
	assertResponse(t, w, http.StatusOK, `{"data":{}}`)
}

func TestResponseCursor(t *testing.T) {
	ctx, w := newTestContext(t, "")
	ResponseCursor(ctx, []testItem{{ID: "a"}, {ID: "b"}}, true, "tok")
	assertResponse(t, w, http.StatusOK,
		`{"data":[{"id":"a"},{"id":"b"}],"pagination":{"has_more":true,"next_cursor":"tok"}}`)
}

func TestResponseCursorLastPage(t *testing.T) {
	ctx, w := newTestContext(t, "")
	ResponseCursor(ctx, []testItem{}, false, "")
	// has_more stays explicit; next_cursor omitted; empty slice → "data":[]
	assertResponse(t, w, http.StatusOK, `{"data":[],"pagination":{"has_more":false}}`)
}

func TestResponseCursorNilSliceNormalized(t *testing.T) {
	ctx, w := newTestContext(t, "")
	// nil is the default Go slice state (var s []T + no appends) — the
	// helper must emit "data":[] rather than "data":null (R1).
	ResponseCursor[testItem](ctx, nil, false, "")
	assertResponse(t, w, http.StatusOK, `{"data":[],"pagination":{"has_more":false}}`)
}

func TestResponseOffset(t *testing.T) {
	ctx, w := newTestContext(t, "")
	ResponseOffset(ctx, []testItem{{ID: "a"}}, 42, 2, 20)
	assertResponse(t, w, http.StatusOK,
		`{"data":[{"id":"a"}],"pagination":{"total":42,"page":2,"page_size":20}}`)
}

func TestResponseOffsetNilSliceNormalized(t *testing.T) {
	ctx, w := newTestContext(t, "")
	ResponseOffset[testItem](ctx, nil, 0, 1, 20)
	assertResponse(t, w, http.StatusOK,
		`{"data":[],"pagination":{"total":0,"page":1,"page_size":20}}`)
}

func TestGetCursorParams(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantCur  string
		wantSize int
	}{
		{"defaults", "", "", 20},
		{"explicit", "cursor=tok&page_size=50", "tok", 50},
		{"size capped at 100", "page_size=1000", "", 100},
		{"invalid size falls back", "page_size=abc", "", 20},
		{"zero size falls back", "page_size=0", "", 20},
		{"negative size falls back", "page_size=-5", "", 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := newTestContext(t, tt.query)
			cursor, size := ctx.GetCursorParams()
			if cursor != tt.wantCur || size != tt.wantSize {
				t.Errorf("GetCursorParams() = (%q, %d), want (%q, %d)", cursor, size, tt.wantCur, tt.wantSize)
			}
		})
	}
}

func TestGetOffsetParams(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantPage int
		wantSize int
	}{
		{"defaults", "", 1, 20},
		{"explicit", "page=3&page_size=50", 3, 50},
		{"zero page falls back", "page=0", 1, 20},
		{"negative page falls back", "page=-2", 1, 20},
		{"invalid page falls back", "page=abc", 1, 20},
		{"size capped at 100", "page=2&page_size=999", 2, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := newTestContext(t, tt.query)
			page, size := ctx.GetOffsetParams()
			if page != tt.wantPage || size != tt.wantSize {
				t.Errorf("GetOffsetParams() = (%d, %d), want (%d, %d)", page, size, tt.wantPage, tt.wantSize)
			}
		})
	}
}

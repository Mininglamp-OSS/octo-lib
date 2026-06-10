package wkhttp

import (
	"net/http"
	"strconv"

	"github.com/Mininglamp-OSS/octo-lib/pkg/envelope"
)

// Envelope response helpers — emit the canonical OCTO API wire shapes
// defined in pkg/envelope (spec rules R1/R5). New endpoints should use
// these instead of the legacy Response / ResponseOK helpers; error
// responses keep going through RenderError / the injected ErrorRenderer.
//
// Go methods cannot take type parameters, so the helpers accept `any`;
// the emitted JSON is identical to the corresponding envelope generic
// (swag annotations should reference envelope.Data[T] etc. for schemas).

// R5 pagination parameter bounds.
const (
	defaultPageSize = 20
	maxPageSize     = 100
	defaultPage     = 1
)

// ResponseData replies 200 with the single-object envelope:
// { "data": data } (R1).
func (c *Context) ResponseData(data any) {
	c.JSON(http.StatusOK, envelope.Data[any]{Data: data})
}

// ResponseCreated replies 201 with the single-object envelope:
// { "data": data } (R1) — for create endpoints returning the new object.
func (c *Context) ResponseCreated(data any) {
	c.JSON(http.StatusCreated, envelope.Data[any]{Data: data})
}

// ResponseEmpty replies 200 with the empty success envelope:
// { "data": {} } (R1) — for delete and state-machine actions.
//
// It intentionally does NOT reuse the legacy ResponseOK name: ResponseOK
// emits {"status":200} and existing clients depend on that shape.
func (c *Context) ResponseEmpty() {
	c.JSON(http.StatusOK, envelope.Data[envelope.EmptyResp]{})
}

// ResponseCursor replies 200 with the cursor-paginated list envelope:
// { "data": items, "pagination": {has_more, next_cursor} } (R1 + R5).
// items must be a slice; pass an empty (non-nil) slice for no results so
// the wire shows "data": [] instead of null. nextCursor is the opaque
// token for the next page ("" on the last page).
func (c *Context) ResponseCursor(items any, hasMore bool, nextCursor string) {
	c.JSON(http.StatusOK, struct {
		Data       any                       `json:"data"`
		Pagination envelope.CursorPagination `json:"pagination"`
	}{
		Data:       items,
		Pagination: envelope.CursorPagination{HasMore: hasMore, NextCursor: nextCursor},
	})
}

// ResponseOffset replies 200 with the offset-paginated list envelope:
// { "data": items, "pagination": {total, page, page_size} } (R1 + R5).
// items must be a slice; pass an empty (non-nil) slice for no results.
func (c *Context) ResponseOffset(items any, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, struct {
		Data       any                       `json:"data"`
		Pagination envelope.OffsetPagination `json:"pagination"`
	}{
		Data:       items,
		Pagination: envelope.OffsetPagination{Total: total, Page: page, PageSize: pageSize},
	})
}

// GetCursorParams reads the R5 cursor-mode query parameters: `cursor`
// (opaque token, "" on first page) and `page_size` (default 20, capped
// at 100; invalid values fall back to the default).
func (c *Context) GetCursorParams() (cursor string, pageSize int) {
	return c.Query("cursor"), c.pageSizeParam()
}

// GetOffsetParams reads the R5 offset-mode query parameters: `page`
// (default 1) and `page_size` (default 20, capped at 100). Invalid
// values fall back to the defaults.
//
// Unlike the legacy GetPage (page_index, default size 15), this follows
// the R5 parameter contract.
func (c *Context) GetOffsetParams() (page, pageSize int) {
	page, _ = strconv.Atoi(c.Query("page"))
	if page <= 0 {
		page = defaultPage
	}
	return page, c.pageSizeParam()
}

func (c *Context) pageSizeParam() int {
	size, _ := strconv.Atoi(c.Query("page_size"))
	if size <= 0 {
		return defaultPageSize
	}
	if size > maxPageSize {
		return maxPageSize
	}
	return size
}

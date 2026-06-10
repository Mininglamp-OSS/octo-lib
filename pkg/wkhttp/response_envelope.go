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
// Single-object helpers are Context methods taking `any`; list helpers
// are package-level generic functions (methods cannot take type
// parameters) so the slice type is compiler-checked and nil slices are
// normalized to []. Swag annotations reference envelope.Data[T] /
// envelope.CursorList[T] / envelope.OffsetList[T] for schemas.

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
// nextCursor is the opaque token for the next page ("" on the last page).
//
// It is a package-level generic function (methods cannot take type
// parameters): the []T parameter rejects non-slice payloads at compile
// time, and a nil slice is normalized so the wire always shows
// "data": [] instead of null.
func ResponseCursor[T any](c *Context, items []T, hasMore bool, nextCursor string) {
	if items == nil {
		items = []T{}
	}
	c.JSON(http.StatusOK, envelope.CursorList[T]{
		Data:       items,
		Pagination: envelope.CursorPagination{HasMore: hasMore, NextCursor: nextCursor},
	})
}

// ResponseOffset replies 200 with the offset-paginated list envelope:
// { "data": items, "pagination": {total, page, page_size} } (R1 + R5).
// See ResponseCursor for why this is a function and how nil is handled.
func ResponseOffset[T any](c *Context, items []T, total int64, page, pageSize int) {
	if items == nil {
		items = []T{}
	}
	c.JSON(http.StatusOK, envelope.OffsetList[T]{
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

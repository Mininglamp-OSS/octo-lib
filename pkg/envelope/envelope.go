// Package envelope defines the canonical OCTO API response wire shapes.
//
// Per the OCTO API spec (octo-openapi-dev-skill, rules R1/R2/R5), every
// endpoint response is wrapped in one of five envelopes:
//
//	Data[T]       { "data": T }
//	CursorList[T] { "data": [T], "pagination": {has_more, next_cursor} }
//	OffsetList[T] { "data": [T], "pagination": {total, page, page_size} }
//	Data[EmptyResp] { "data": {} }            — delete / state-machine success
//	Error         { "error": {code, message, details, hint} }
//
// The package is dependency-free on purpose: services that do not use
// pkg/wkhttp (plain gin or otherwise) can still reference these types in
// swag annotations and response construction. pkg/wkhttp provides
// Context helpers (ResponseData / ResponseCursor / ...) that emit these
// exact shapes.
//
// The contract is the WIRE SHAPE, not the type names — linting validates
// the generated OpenAPI structure (top-level `data` / `error`), so any
// implementation producing the same JSON is compliant. These types are
// the recommended shared implementation.
package envelope

// Data is the single-object success envelope (R1): { "data": T }.
// Use it for get / create / update responses, and Data[EmptyResp] for
// success responses that carry no payload.
type Data[T any] struct {
	Data T `json:"data"`
}

// EmptyResp marshals to {}. Data[EmptyResp] renders { "data": {} } — the
// canonical success shape for delete and state-machine actions (R1).
type EmptyResp struct{}

// CursorPagination is the cursor-mode pagination block (R5).
// NextCursor is an opaque server token; it is omitted when there is no
// further page. Cursor mode must NOT include a total count.
type CursorPagination struct {
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// OffsetPagination is the offset-mode pagination block (R5).
type OffsetPagination struct {
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// CursorList is the cursor-paginated list envelope (R1 + R5):
// { "data": [T], "pagination": {has_more, next_cursor} }.
type CursorList[T any] struct {
	Data       []T              `json:"data"`
	Pagination CursorPagination `json:"pagination"`
}

// OffsetList is the offset-paginated list envelope (R1 + R5):
// { "data": [T], "pagination": {total, page, page_size} }.
type OffsetList[T any] struct {
	Data       []T              `json:"data"`
	Pagination OffsetPagination `json:"pagination"`
}

// Error is the failure envelope for all 4xx/5xx responses (R1 + R2):
// { "error": {code, message, details, hint} }.
//
// This is the spec shape; a service may render a superset for legacy
// compatibility (e.g. octo-server's D14 renderer adds msg/status
// siblings) — the contract only requires the top-level `error` object.
type Error struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody carries the R2 error fields. Code is one of the 12 fixed
// enum values (or a service-defined registry id during migration);
// Details is structured, machine-readable sub-classification; Hint is an
// optional human-readable recovery suggestion.
type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	Hint    string         `json:"hint,omitempty"`
}

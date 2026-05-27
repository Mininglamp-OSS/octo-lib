package wkhttp

import (
	"net/http"
	"net/http/httptest"
)

// runMiddleware 在隔离的 *WKHttp 上挂载单个中间件，触发请求，写到 w。
// 中间件 abort 时不会进入终端 handler；未 abort 则会进入 noop handler。
func runMiddleware(wk *WKHttp, mw HandlerFunc, w *httptest.ResponseRecorder, req *http.Request) {
	runChain(wk, []HandlerFunc{mw, noopHandler}, w, req)
}

// runChain 顺序挂载多个 HandlerFunc 到 /，触发请求。
func runChain(wk *WKHttp, chain []HandlerFunc, w *httptest.ResponseRecorder, req *http.Request) {
	wk.GET("/", chain...)
	req.URL.Path = "/"
	wk.ServeHTTP(w, req)
}

func noopHandler(c *Context) {}

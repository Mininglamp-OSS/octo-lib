package wkhttp

import (
	"context"
	"testing"
)

func TestWithUserAndUserFromCtx(t *testing.T) {
	info := UserInfo{UID: "u1", Name: "alice", Role: "admin", Language: "zh-CN"}
	ctx := WithUser(context.Background(), info)

	got, ok := UserFromCtx(ctx)
	if !ok {
		t.Fatal("UserFromCtx returned ok=false")
	}
	if got != info {
		t.Errorf("got %+v, want %+v", got, info)
	}
}

func TestUserFromCtxMissing(t *testing.T) {
	if _, ok := UserFromCtx(context.Background()); ok {
		t.Error("UserFromCtx on empty ctx returned ok=true")
	}
	if _, ok := UserFromCtx(nil); ok {
		t.Error("UserFromCtx(nil) returned ok=true")
	}
}

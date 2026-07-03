package config

import (
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Mininglamp-OSS/octo-lib/pkg/network"
	"github.com/sendgrid/rest"
)

// IMObserver 接收一次 WuKongIM 出站调用的耗时。op 是低基数枚举（见文件末尾 opXxx
// 常量），标识调用了哪个 IM 端点；dur 为调用耗时；err 为调用结果分类（nil=成功）。
//
// 与 pkg/db.DBObserver / pkg/redis.RedisObserver 同构：octo-lib 不依赖 Prometheus，
// 只暴露这个回调，由 octo-server 在启动时用 SetIMObserver 注入真实现（接 #440 的
// DependencyMetrics，label dependency="wukongim"）。实现方须轻量且不 panic —— 它在
// 每次 IM 调用的热路径上同步执行（消息收发路径）。
type IMObserver func(op string, dur time.Duration, err error)

// imObserver 持有进程级 observer。用 atomic.Pointer：启动时 SetIMObserver 设置一次，
// 业务热路径只读；未注入（指标关闭 / 单测）时 reportIM 为安全 no-op，绝不 panic。
var imObserver atomic.Pointer[IMObserver]

// SetIMObserver 注入进程级 IM observer。传 nil 可清除（恢复 no-op）。
// 通常在 main 启动早期调用一次。
func SetIMObserver(o IMObserver) {
	if o == nil {
		imObserver.Store(nil)
		return
	}
	imObserver.Store(&o)
}

// reportIM 把一次调用转发给已注入的 observer；未注入时为 no-op。
//
// 在接缝处兜住下游 observer 的 panic：observer 跑在每次 IM 调用的热路径上，一个
// nil-deref 不该顺着消息收发炸上来，最多丢掉这一条埋点样本。godoc 已要求实现方
// 「不 panic」，这里再加一道防线而非纯靠信任（与 pkg/db.reportDB 一致）。
func reportIM(op string, dur time.Duration, err error) {
	defer func() { _ = recover() }()
	if p := imObserver.Load(); p != nil {
		(*p)(op, dur, err)
	}
}

// errIMBadStatus 是「IM 返回非 200」这一分类的哨兵错误，仅用于 observer 的 ok/error
// 判定，不回传给调用方（保持调用透明）。用哨兵而非 fmt.Errorf(状态码) 既避免把状态码
// 带成高基数，也避免错误路径的每次分配 —— 指标只区分 ok/error，不记录具体状态码。
var errIMBadStatus = errors.New("wukongim: non-200 response")

// imCallErr 把一次 IM 调用的 (resp, err) 归一化为「成功/失败」分类，供 reportIM 打
// status label。判定与本包 handlerIMError 一致：传输错误、无响应、或非 200 都算
// error。注意 4xx（含调用方参数错误）也计入 error —— 与各方法自身对成功的定义
// （StatusCode==200）保持同一口径，宁可统一，也不在指标层另立一套判定。
func imCallErr(resp *rest.Response, err error) error {
	if err != nil {
		return err
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		return errIMBadStatus
	}
	return nil
}

// imPost 对一次 WuKongIM POST 调用计时并上报，返回 network.Post 的 (resp, err) 原样
// 不改（透明）。所有 IM POST 端点都应经由此方法调用，以获得统一的 dependency 指标。
func (c *Context) imPost(op, path string, body []byte) (*rest.Response, error) {
	start := time.Now()
	resp, err := network.Post(c.cfg.WuKongIM.APIURL+path, body, c.wkIMManagerTokenHeader())
	reportIM(op, time.Since(start), imCallErr(resp, err))
	return resp, err
}

// imGet 对一次 WuKongIM GET 调用计时并上报，语义同 imPost。
func (c *Context) imGet(op, path string, queryParams map[string]string) (*rest.Response, error) {
	start := time.Now()
	resp, err := network.Get(c.cfg.WuKongIM.APIURL+path, queryParams, c.wkIMManagerTokenHeader())
	reportIM(op, time.Since(start), imCallErr(resp, err))
	return resp, err
}

// op 是 dependency="wukongim" 指标的低基数 op label 取值集合。手工映射到 IM 端点，
// 绝不从 URL 派生 —— 这是 Prometheus 序列基数的护栏。新增 IM 端点时在此登记一个常量。
const (
	// user / token
	opUpdateUserToken  = "update_user_token"
	opQuitUserDevice   = "quit_user_device"
	opUserOnlineStatus = "user_online_status"

	// message
	opSendMessage        = "send_message"
	opSendMessageBatch   = "send_message_batch"
	opSyncMessage        = "message_sync"
	opSyncMessageAck     = "message_syncack"
	opRevokeMessage      = "message_revoke"
	opGetChannelMessages = "get_channel_messages"
	opSearchMessages     = "search_messages"
	opSearchUserMessages = "user_search"
	opSyncChannelMessage = "channel_message_sync"

	// channel
	opChannelInfo      = "channel_info"
	opChannelCreate    = "channel_create"
	opChannelDelete    = "channel_delete"
	opChannelMaxSeq    = "channel_max_seq"
	opBlacklistAdd     = "blacklist_add"
	opBlacklistSet     = "blacklist_set"
	opBlacklistRemove  = "blacklist_remove"
	opWhitelistAdd     = "whitelist_add"
	opWhitelistSet     = "whitelist_set"
	opWhitelistRemove  = "whitelist_remove"
	opSubscriberAdd    = "subscriber_add"
	opSubscriberRemove = "subscriber_remove"

	// conversation
	opGetConversations      = "get_conversations"
	opConversationSetUnread = "conversation_set_unread"
	opConversationDelete    = "conversation_delete"
	opConversationSync      = "conversation_sync"

	// stream
	opStreamStart = "stream_start"
	opStreamEnd   = "stream_end"
)

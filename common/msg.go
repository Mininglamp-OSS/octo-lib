package common

import (
	"fmt"
	"strings"

	imlog "github.com/Mininglamp-OSS/octo-lib/pkg/log"
	"github.com/Mininglamp-OSS/octo-lib/pkg/util"
	"go.uber.org/zap"
)

// ContentType 正文类型
type ContentType int

const (
	// ---------- 聊天类 ----------

	// Text 文本消息
	Text ContentType = 1 // 文本消息
	// Image 图片消息
	Image ContentType = 2
	// GIF 消息
	GIF ContentType = 3
	//Voice 语音消息
	Voice ContentType = 4
	// Video 视频
	Video ContentType = 5
	// LOCATION 位置
	Location ContentType = 6
	// Card 名片
	Card ContentType = 7
	// File 文件
	File ContentType = 8
	// MultipleForward 合并转发
	MultipleForward ContentType = 11
	// VectorSticker 矢量表情
	VectorSticker ContentType = 12
	// EmojiSticker 矢量emoji表情
	EmojiSticker ContentType = 13
	// RichText 富文本消息
	RichText ContentType = 14
	// Forward 转发消息
	// InviteJoinOrganization 邀请加入组织
	InviteJoinOrganization ContentType = 16

	// 消息正文错误
	ContentError ContentType = 97
	// signal 解密失败
	SignalError ContentType = 98
	// CMD 消息
	CMD ContentType = 99

	// ---------- 系统类 ----------
	// Tip 只作为提醒无任何操作类型
	Tip ContentType = 2000
	// FriendApply 好友申请
	FriendApply ContentType = 1000
	// GroupCreate 群创建
	GroupCreate ContentType = 1001
	// GroupMemberAdd 群成员添加
	GroupMemberAdd ContentType = 1002
	// GroupMemberRemove  群成员移除
	GroupMemberRemove ContentType = 1003

	// FriendSure 好友申请
	FriendSure ContentType = 1004
	// GroupUpdate 群更新
	GroupUpdate ContentType = 1005
	// RevokeMessage 撤回消息
	RevokeMessage ContentType = 1006
	// GroupMemberScanJoin 扫码进群
	GroupMemberScanJoin ContentType = 1007
	// GroupTransferGrouper 转让群主
	GroupTransferGrouper ContentType = 1008
	// GroupMemberInvite 群成员邀请
	GroupMemberInvite ContentType = 1009
	// GroupMemberBeRemove  群成员被移除（被踢）
	GroupMemberBeRemove ContentType = 1020
	// GroupMemberBeRemove 群成员主动退出群聊
	GroupMemberQuit ContentType = 1021
	// 群升级
	GroupUpgrade ContentType = 1022

	// ---------- 客服类 ----------
	HotlineAssignTo ContentType = 1200 // 分配客服
	HotlineSolved   ContentType = 1201 // 已解决
	HotlineReopen   ContentType = 1202 // 会话被重开

	// ---------- 音视频 ----------
	VideoCallResult ContentType = 9989 // 音视频通话结果
)

func GetDisplayText(contentType int) string {
	if contentType == Text.Int() {
		return "文本消息"
	} else if contentType == Image.Int() {
		return "图片消息"
	} else if contentType == GIF.Int() {
		return "GIF"
	} else if contentType == Voice.Int() {
		return "语音"
	} else if contentType == Video.Int() {
		return "视频"
	} else if contentType == Location.Int() {
		return "位置"
	} else if contentType == Card.Int() {
		return "名片"
	} else if contentType == File.Int() {
		return "文件"
	} else if contentType == MultipleForward.Int() {
		return "合并转发消息"
	} else if contentType == VectorSticker.Int() {
		return "贴纸"
	} else if contentType == EmojiSticker.Int() {
		return "emoji"
	} else if contentType == RichText.Int() {
		return "富文本消息"
	}
	return "未知消息类型"
}
func (c ContentType) String() string {
	switch c {
	case Text:
		return "Text"
	case Image:
		return "Image"
	case GIF:
		return "GIF"
	case Voice:
		return "Voice"
	case CMD:
		return "CMD"
	case FriendApply:
		return "FriendApply"
	case GroupCreate:
		return "GroupCreate"
	case GroupMemberAdd:
		return "GroupMemberAdd"
	case GroupMemberRemove:
		return "GroupMemberRemove"
	case FriendSure:
		return "FriendSure"
	case GroupUpdate:
		return "GroupUpdate"
	case RevokeMessage:
		return "RevokeMessage"
	}
	return fmt.Sprintf("%d", c)
}

// Int Int
func (c ContentType) Int() int {
	return int(c)
}

// GetFakeChannelIDWith GetFakeChannelIDWith
func GetFakeChannelIDWith(fromUID, toUID string) string {
	// TODO：这里可能会出现相等的情况 ，如果相等可以截取一部分再做hash直到不相等，后续完善
	fromUIDHash := util.HashCrc32(fromUID)
	toUIDHash := util.HashCrc32(toUID)
	if fromUIDHash > toUIDHash {
		return fmt.Sprintf("%s@%s", fromUID, toUID)
	}
	if fromUIDHash == toUIDHash {
		imlog.Warn("生成的fromUID的Hash和toUID的Hash是相同的！！", zap.Uint32("fromUIDHash", fromUIDHash), zap.Uint32("toUIDHash", toUIDHash), zap.String("fromUID", fromUID), zap.String("toUID", toUID))
	}

	return fmt.Sprintf("%s@%s", toUID, fromUID)
}

// IsFakeChannel 是fake频道
func IsFakeChannel(channelID string) bool {
	return strings.Contains(channelID, "@")
}

// 获取fakeChannelID里的非uid的uid
func GetToChannelIDWithFakeChannelID(fakeChannelID string, uid string) string {
	channelIDs := strings.Split(fakeChannelID, "@")
	toChannelID := fakeChannelID
	if len(channelIDs) == 2 {
		if channelIDs[0] == uid {
			toChannelID = channelIDs[1]
		} else {
			toChannelID = channelIDs[0]
		}
	}
	return toChannelID
}

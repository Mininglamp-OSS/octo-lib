package model

import "github.com/Mininglamp-OSS/octo-lib/config"

type ChannelResp struct {
	Channel struct {
		ChannelID   string `json:"channel_id"`
		ChannelType uint8  `json:"channel_type"`
	} `json:"channel"`
	ParentChannel *struct {
		ChannelID   string `json:"channel_id"`
		ChannelType uint8  `json:"channel_type"`
	} `json:"parent_channel,omitempty"`
	Username    string            `json:"username,omitempty"` // 频道唯一标识（目前只有机器人有用到）
	Name        string            `json:"name"`               // 频道名称
	Logo        string            `json:"logo"`               // 频道logo
	Remark      string            `json:"remark"`             // 频道备注
	Status      int               `json:"status"`             //  频道状态 0.正常 1.正常  2.黑名单
	Online      int               `json:"online"`             // 是否在线
	LastOffline int64             `json:"last_offline"`       // 最后一次离线
	DeviceFlag  config.DeviceFlag `json:"device_flag"`        // 设备标记
	Receipt     int               `json:"receipt"`            // 消息是否回执
	Robot       int               `json:"robot"`              // 是否是机器人
	Category    string            `json:"category"`           // 频道类别
	// 设置
	Stick    int `json:"stick"`     // 是否置顶
	Mute     int `json:"mute"`      // 是否免打扰
	ShowNick int `json:"show_nick"` // 是否显示昵称
	// 个人特有
	Follow      int `json:"follow"`       // 是否已关注 0.未关注（陌生人） 1.已关注（好友）
	BeDeleted   int `json:"be_deleted"`   // 是否被对方删除
	BeBlacklist int `json:"be_blacklist"` // 是否被对方拉入黑名单
	// 群特有
	Notice    string `json:"notice"`    // 群公告
	Save      int    `json:"save"`      // 群是否保存
	Forbidden int    `json:"forbidden"` // 群是否全员禁言
	Invite    int    `json:"invite"`    // 是否开启邀请

	Flame       int `json:"flame"`        // 阅后即焚
	FlameSecond int `json:"flame_second"` // 阅后即焚秒数

	// 此内容在扩展内容内
	// Screenshot          int `json:"screenshot"`             // 是否开启截屏通知
	// ForbiddenAddFriend  int `json:"forbidden_add_friend"`   // 是否禁止群内添加好友
	// JoinGroupRemind     int `json:"join_group_remind"`      // 是否开启进群提醒
	// RevokeRemind        int `json:"revoke_remind"`          // 是否开启撤回通知
	// chatPwdOn           int `json:"chat_pwd_on"`            // 是否开启聊天密码
	// AllowViewHistoryMsg int `json:"allow_view_history_msg"` // 是否允许新成员查看群历史记录

	Extra map[string]interface{} `json:"extra"` // 扩展内容
}

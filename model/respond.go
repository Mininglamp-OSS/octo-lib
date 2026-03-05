package model

type Invite struct {
	InviteCode string
	Vercode    string
	Uid        string
}

type FriendResp struct {
	Remark    string // 好友备注
	ToUID     string // 好友uid
	IsDeleted int    // 是否删除
	IsAlone   int    // 是否为单项好友
	ShortNo   string // 短编号
}

type GroupMemberResp struct {
	GroupNo            string
	UID                string
	Name               string // 群成员名称
	Remark             string // 群成员备注
	InviteUID          string // 邀请人uid
	IsDeleted          int    // 是否删除
	Role               int    // 成员角色 1. 创建者	 2.管理员
	Status             int    // 成员状态 0.禁用 1.正常，2.黑名单
	CreatedAt          string // 加入时间
	ForbiddenExpirTime int64  // 禁言时长
}

type DeviceResp struct {
	ID          int64  // 设备id
	DeviceID    string // 设备号
	DeviceName  string // 设备名称
	UID         string // 用户id
	DeviceModel string // 设备型号
}
type CallingChannelResp struct {
	ChannelID    string // 通话频道id
	ChannelType  uint8  // 通话频道类型
	RoomName     string // 房间名称
	Participants []*BaseUserVO
}
type BaseUserVO struct {
	UID  string // 用户id
	Name string // 用户名称
}

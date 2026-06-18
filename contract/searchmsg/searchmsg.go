// Package searchmsg 定义 octo-im 消息检索管线的 Kafka 契约 —— 单一真源。
//
// 该契约被两侧 import：octo-server 的 searchetl producer（往 Kafka 写），以及
// 独立镜像 es-indexer consumer（从 Kafka 读、bulk 写 OpenSearch）。把 struct 放在
// octo-lib（三仓共享 config 宿主）是硬要求：若放 octo-server，独立镜像的 es-indexer
// 无法 import，会出现「编译过但运行期字段错位静默吃数据」。
//
// 设计纪律：
//   - 字段名与 message 表/分表细节**解耦**：契约只描述「一条可检索的消息正文」，不暴露
//     分表名、payload 内部结构、自增 id 等存储实现。未来源头从 ETL 升级到 Outbox+CDC，
//     只需换「谁往 Kafka 写」，下游零改。
//   - 带 SchemaVersion：consumer 见未知版本必须进 DLQ，**不得静默吃**。
//   - 撤回/删除态**绝不进**该契约（路线甲：读时回 MySQL join 过滤）。这里只承载正文 +
//     查询侧鉴权所需的可见性字段（ChannelID/ChannelType/FromUID/SpaceID/Visibles/MessageSeq）。
package searchmsg

// SchemaVersion 是当前正文契约的版本号。每次对 Message 做不兼容变更（删字段/改语义/
// 改类型）必须 +1；consumer 收到非本值的消息一律进 DLQ，不得按旧结构强解。
//
// v2（本次）：新增 reader 必读的安全/正确性字段 SpaceID / Visibles / MessageSeq。
// 三字段与 octo-search-indexer esindex.Doc（spaceId/visibles/messageSeq）逐字段对齐：
// indexer 的 SafetyFieldsSchemaVersion==2 + LiveContractCarriesSafetyFields()（仅判
// searchmsg.SchemaVersion >= 2）据此在 indexer 重 pin 到本契约后自动解封实时写入封锁
// （否则空 visibles → reader fail-OPEN，群系统消息普通成员能搜出群管才可见消息）。
const SchemaVersion = 2

// SourceETLMessageTable 是 ETL（读 message 表）阶段的 Source 取值。
// 未来升级到 Outbox+CDC 时改该值（如 "cdc-binlog"），下游据此区分来源但不改解析逻辑。
const SourceETLMessageTable = "etl-message-table"

// Message 是 Kafka topic `octo.message.v1` 的正文契约（JSON 编解码）。
//
// Kafka key = MessageID（保证同一消息进同一分区、ES _id upsert 幂等）。
// ES doc _id = MessageID。
//
// ⚠️ MessageID 显式为 string：对应 message 表的 VARCHAR(20) message_id 列。严禁参考
// base/elastic stub 里的 uint64 类型（那是与实际 schema 双重错配的死代码）。
type Message struct {
	// SchemaVersion 契约版本，= 包级常量 SchemaVersion。consumer 必校验。
	SchemaVersion int `json:"schema_version"`

	// MessageID 消息唯一 id（= Kafka key = ES _id），对应 message 表 VARCHAR(20) message_id。
	MessageID string `json:"message_id"`

	// ChannelID 频道 id。群=groupNo；话题=groupNo____shortID；私聊=uid1@uid2 假频道 id。
	// 查询侧鉴权 filter 与 DM 参与方判定依赖此字段，必存。
	ChannelID string `json:"channel_id"`
	// ChannelType 频道类型，见 octo-lib common.ChannelType（None/Person/Group/...）。
	// 查询侧鉴权 filter 据此分流，必存。
	ChannelType int `json:"channel_type"`
	// FromUID 发送者 uid。
	FromUID string `json:"from_uid"`

	// SpaceID 空间隔离 id（对齐 indexer esindex.Doc.SpaceID `spaceId`）。p2p(DM) 召回
	// 过滤依赖此字段；reader 对空 SpaceID 走 fail-closed（同 space 也 0 命中），故 producer
	// 富化前实时路径写空属安全方向。omitempty 与 indexer 一致。
	SpaceID string `json:"space_id,omitempty"`
	// Visibles 群消息可见性白名单（对齐 indexer esindex.Doc.Visibles `visibles`）。群系统
	// 消息「仅管理员可见」gate 强依赖此字段；reader 对空 Visibles 是 **fail-OPEN**（普通成员
	// 能搜出群管才可见消息），故必须由 producer 富化后下游才能安全启用实时写入。omitempty 与
	// indexer 一致。
	Visibles []string `json:"visibles,omitempty"`

	// Content 消息正文（从 payload 解出的可检索文本）。
	// 当 RawExcluded=true（Signal 加密 / 非文本类）时为 nil（不算丢消息）。
	Content *string `json:"content"`
	// ContentType 消息内容类型（payload.type，规约为 int）。
	ContentType int `json:"content_type"`

	// RawExcluded 标记「已知不可索引类」：Signal 加密 DM（payload 非明文）或非文本结构化
	// 内容。为 true 时 Content 置 nil，走正常流不进 DLQ —— 与「本应可解析却解析失败的真
	// 异常」（进 DLQ）严格区分。
	RawExcluded bool `json:"raw_excluded"`

	// MsgTimestamp 消息发送时间（纪元秒，= message.timestamp）。
	MsgTimestamp int64 `json:"msg_timestamp"`
	// CreatedAt 落库时间（纪元秒，= UNIX_TIMESTAMP(message.created_at)）。
	CreatedAt int64 `json:"created_at"`

	// MessageSeq 频道内消息序号（取自 message.message_seq 列；对齐 indexer
	// esindex.Doc.MessageSeq `messageSeq`，uint64 全精度）。reader channel_offset
	// 「清空会话」gate（visibility.go）依赖此字段，缺/0 则保守隐藏（安全方向）。omitempty
	// 与 indexer 一致。
	MessageSeq uint64 `json:"message_seq,omitempty"`

	// Source 数据来源标识，ETL 阶段 = SourceETLMessageTable；CDC 阶段换值，下游不改解析。
	Source string `json:"source"`
}

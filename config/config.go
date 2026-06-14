package config

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Mininglamp-OSS/octo-lib/pkg/util"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"
)

type Mode string

const (
	//debug 模式
	DebugMode Mode = "debug"
	// 正式模式
	ReleaseMode Mode = "release"
	// 压力测试模式
	BenchMode Mode = "bench"
)

// FileService FileService
type FileService string

const (
	// FileServiceAliyunOSS 阿里云oss上传服务
	FileServiceAliyunOSS FileService = "aliyunOSS"
	// FileServiceSeaweedFS seaweedfs(https://github.com/chrislusf/seaweedfs)
	FileServiceSeaweedFS FileService = "seaweedFS"
	// FileServiceMinio minio
	FileServiceMinio FileService = "minio"
	// FileServiceQiniu 七牛云上传服务
	FileServiceQiniu FileService = "qiniu"
	// FileServiceTencentCOS 腾讯云COS上传服务
	FileServiceTencentCOS FileService = "tencentCOS"
)

func (u FileService) String() string {
	return string(u)
}

type TablePartitionConfig struct {
	MessageTableCount         int // 消息表数量
	MessageUserEditTableCount int // 用户消息编辑表
	ChannelOffsetTableCount   int // 频道偏移表
}

func newTablePartitionConfig() TablePartitionConfig {

	return TablePartitionConfig{
		MessageTableCount:         5,
		MessageUserEditTableCount: 3,
		ChannelOffsetTableCount:   3,
	}
}

// Config 配置信息
type Config struct {
	vp *viper.Viper // 内部配置对象

	// ---------- 基础配置 ----------
	Mode                        Mode   // 模式 debug 测试 release 正式 bench 压力测试
	AppID                       string // APP ID
	AppName                     string // APP名称
	Version                     string // 版本
	RootDir                     string // 数据根目录
	Addr                        string // 服务监听地址 x.x.x.x:8080
	GRPCAddr                    string // grpc的通信地址 （建议内网通信）
	SSLAddr                     string // ssl 监听地址
	MessageSaveAcrossDevice     bool   // 消息是否跨设备保存（换设备登录消息是否还能同步到老消息）
	WelcomeMessage              string // 登录注册欢迎语
	PhoneSearchOff              bool   // 是否关闭手机号搜索
	OnlineStatusOn              bool   // 是否开启在线状态显示
	GroupUpgradeWhenMemberCount int    // 当成员数量大于此配置时 自动升级为超级群 默认为 1000
	EventPoolSize               int64  // 事件任务池大小
	AdminPwd                    string // 后台管理默认密码
	// ---------- 外网配置 ----------
	External struct {
		IP          string // 外网IP
		BaseURL     string // 本服务的对外的基础地址
		H5BaseURL   string // h5页面的基地址 如果没有配置默认未 BaseURL + /web
		APIBaseURL  string // api的基地址 如果没有配置默认未 BaseURL + /v1
		WebLoginURL string // web登录地址
	}
	// ---------- 日志配置 ----------
	Logger struct {
		Dir     string // 日志存储目录
		Level   zapcore.Level
		LineNum bool // 是否显示代码行数
	}
	// ---------- db相关配置 ----------
	DB struct {
		MySQLAddr                  string        // mysql的连接信息
		MySQLMaxOpenConns          int           // 最大连接数
		MySQLMaxIdleConns          int           // 最大空闲连接数
		MySQLConnMaxLifetime       time.Duration // 连接最大生命周期
		Migration                  bool          // 是否合并数据库
		RedisAddr                  string        // redis地址
		RedisPass                  string        // redis密码
		RedisTLS                   bool          // 是否启用 TLS 连接 Redis（rediss://），用于 AWS ElastiCache / Azure Cache 等托管服务
		RedisTLSInsecureSkipVerify bool          // 是否跳过 TLS 证书校验（仅用于测试 / 自签名证书场景）
		RedisTLSCAFile             string        // 自定义 CA 证书文件路径（可选）
		AsynctaskRedisAddr         string        // 异步任务的redis地址 不写默认为RedisAddr的地址
	}
	// ---------- 分布式配置 ----------
	Cluster struct {
		NodeID int //  节点ID 节点ID需要小于1024
	}

	// ---------- 缓存配置 ----------
	Cache struct {
		TokenCachePrefix            string        // token缓存前缀
		LoginDeviceCachePrefix      string        // 登录设备缓存前缀
		LoginDeviceCacheExpire      time.Duration // 登录设备缓存过期时间
		UIDTokenCachePrefix         string        // uidtoken缓存前缀
		FriendApplyTokenCachePrefix string        // 申请好友的token的前缀
		FriendApplyExpire           time.Duration // 好友申请过期时间
		TokenExpire                 time.Duration // token失效时间
		NameCacheExpire             time.Duration // 名字缓存过期时间
	}
	// ---------- 系统账户设置 ----------
	Account struct {
		SystemUID       string //系统账号uid
		FileHelperUID   string // 文件助手uid
		SystemGroupID   string //系统群ID 需求在app_config表里设置new_user_join_system_group为1才有效
		SystemGroupName string // 系统群的名字
		AdminUID        string //系统管理员账号
	}

	// ---------- 文件服务 ----------

	FileService FileService   // 文件服务
	OSS         OSSConfig     // 阿里云oss配置
	Minio       MinioConfig   // minio配置
	Seaweed     SeaweedConfig // seaweedfs配置
	Qiniu       QiniuConfig   // 七牛云配置
	COS         COSConfig     // 腾讯云COS配置
	S3          S3Config      // S3 / S3兼容存储配置（AWS S3、Cloudflare R2、Backblaze B2、Wasabi 等）

	// ---------- 短信运营商 ----------
	SMSCode                string // 模拟的短信验证码
	SMSProvider            SMSProvider
	UniSMS                 UnismsConfig                 // unisms https://unisms.apistd.com/
	AliyunSMS              AliyunSMSConfig              // aliyun sms
	AliyunInternationalSMS AliyunInternationalSMSConfig // 阿里云国际短信
	Smsbao                 SmsbaoConfig                 // 短信宝
	// ---------- 悟空IM ----------
	WuKongIM struct {
		APIURL       string // im基地址
		ManagerToken string // im的管理者token wukongim配置了就需要填写，没配置就不需要
	}
	// ---------- 头像 ----------
	Avatar struct {
		Default        string // 默认头像
		DefaultCount   int    // 默认头像数量
		Partition      int    // 头像分区数量
		DefaultBaseURL string // 默认头像的基地址
	}
	// ---------- 短编号 ----------
	ShortNo struct {
		NumOn   bool // 是否开启数字短编号
		NumLen  int  // 数字短编号长度
		EditOff bool // 是否关闭短编号编辑
	}
	// ---------- robot ----------
	Robot struct {
		MessageExpire      time.Duration // 消息过期时间
		InlineQueryTimeout time.Duration // inlineQuery事件过期时间
		EventPoolSize      int64         // 机器人事件池大小
	}

	// ---------- gitee ----------
	Gitee struct {
		OAuthURL     string // gitee oauth url
		ClientID     string // gitee client id
		ClientSecret string // gitee client secret
	}
	// ---------- github ----------
	Github struct {
		OAuthURL     string // github oauth url
		ClientID     string // github client id
		ClientSecret string // github client secret
	}
	// ---------- owt ----------
	OWT struct {
		URL          string // owt api地址 例如： https://xx.xx.xx.xx:3000/v1
		ServiceID    string // owt的服务ID
		ServiceKey   string // owt的服务key （用户访问后台的api）
		RoomMaxCount int    // 房间最大参与人数
	}
	// ---------- livekit ----------
	Livekit struct {
		Off                  bool   // 是否关闭livekit功能
		URL                  string // livekit连接地址 wss://xx.xx.xx.xx:7880
		APIKey               string // livekit api key
		APISecret            string // livekit api secret
		InviteTimeoutSeconds int    // 邀请超时时间（单位秒）
		RoomMaxCount         int    // 房间最大参与人数
	}
	// ---------- 注册 ----------
	Register struct {
		Off           bool // 是否关闭注册
		OnlyChina     bool // 是否仅仅中国手机号可以注册
		StickerAddOff bool // 是否关闭注册添加表情
		UsernameOn    bool // 是否开启用户名注册
		EmailOn       bool // 是否开启邮箱注册/登录
	}
	Organization struct {
		ImportOn bool // 是否开启导入组织信息
	}
	// ---------- 好友 ----------
	Friend struct {
		AddedTipsText string // 成为好友系统提示消息
	}
	// ---------- 消息搜索 ----------
	ZincSearch struct {
		SearchOn           bool   // 是否开启消息搜索
		APIURL             string // ZincSearch 请求地址
		ZincUsername       string // ZincSearch 登录用户名
		ZincPassword       string // ZincSearch 登录密码
		SyncIntervalSecond int    // 同步消息间隔时间（单位秒）
		SyncCount          int    // 每张表每次同步数量 默认100条
	}
	// ---------- 消息检索读侧后端切换（YUJ-4530 ETL→Kafka→ES indexer） ----------
	// ReadBackend 选择检索读路径走哪个后端："zinc"（ZincSearch，默认/现状）或 "es"（OpenSearch + 查询侧 join）。
	// 双写灰度期保持 ZincSearch 写入开启，仅切读；稳定后才下线 Zinc 写。
	Search struct {
		ReadBackend string // "zinc" | "es"，默认 "zinc"
	}
	// ---------- Kafka 共享总线（searchetl producer / es-indexer consumer） ----------
	// 契约 struct 单一真源见 octo-lib contract/searchmsg。两侧 import 同一包。
	Kafka struct {
		On       bool     // 是否开启 Kafka 接入（producer 侧；关则 searchetl 仅空跑游标不投递）
		Brokers  []string // broker 地址列表，如 ["kafka:9092"]
		Topic    string   // 正文 topic，默认 octo.message.v1
		DLQTopic string   // 死信 topic，默认 octo.message.v1.dlq
	}
	// ---------- es-indexer 的 OpenSearch 写入（独立镜像 binary 用，不复用 ElasticsearchURL/olivere v6） ----------
	ESIndex struct {
		On       bool     // 是否开启 OpenSearch bulk 写入
		Addrs    []string // OpenSearch 节点地址列表，如 ["https://opensearch:9200"]
		Username string   // 基础认证用户名
		Password string   // 基础认证密码
		Index    string   // 索引名（写读同名/或读用别名），默认 octo-message
	}
	// ---------- 群 ----------
	Group struct {
		SameDayCreateMaxCount     int  // 同一天创建群的最大数量
		CreateGroupVerifyFriendOn bool // 建群是否开启好友验证
	}
	// ---------- push ----------
	Push struct {
		ContentDetailOn bool         //  推送是否显示正文详情(如果为false，则只显示“您有一条新的消息” 默认为true)
		PushPoolSize    int64        // 推送任务池大小
		APNS            APNSPush     // 苹果推送
		MI              MIPush       // 小米推送
		HMS             HMSPush      // 华为推送
		VIVO            VIVOPush     // vivo推送
		OPPO            OPPOPush     // oppo推送
		FIREBASE        FIREBASEPush // FIREBASE推送
	}
	// ---------- message ----------
	Message struct {
		SendMessageOn                 bool // 是否开启接口发送发送消息
		SyncReadedCountIntervalSecond int  // 同步消息已读数量间隔时间（单位秒）
	}
	// ---------- wechat ----------
	Wechat struct {
		AppID     string // 微信appid 在开放平台内
		AppSecret string
	}

	// ---------- tracing ----------
	Tracing struct {
		On   bool   // 是否开启tracing
		Addr string // tracer的地址
	}

	// ---------- support ----------
	Support struct {
		Email     string // 技术支持的邮箱地址
		EmailSmtp string // 技术支持的邮箱的smtp
		EmailPwd  string // 邮箱密码
	}

	// ---------- 其他 ----------

	Test bool // 是否是测试模式

	QRCodeInfoURL    string   // 获取二维码信息的URL
	VisitorUIDPrefix string   // 访客uid的前缀
	TimingWheelTick  duration // The time-round training interval must be 1ms or more
	TimingWheelSize  int64    // Time wheel size

	ElasticsearchURL string // elasticsearch 地址

	TablePartitionConfig TablePartitionConfig

	// ---------- 系统配置  由系统生成,无需用户配置 ----------
	AppRSAPrivateKey string
	AppRSAPubKey     string
}

// New New
func New() *Config {
	cfg := &Config{
		// ---------- 基础配置 ----------
		Mode:                        ReleaseMode,
		AppID:                       "tangsengdaodao",
		AppName:                     "唐僧叨叨",
		Addr:                        ":8090",
		GRPCAddr:                    "0.0.0.0:6979",
		PhoneSearchOff:              false,
		OnlineStatusOn:              true,
		GroupUpgradeWhenMemberCount: 1000,
		MessageSaveAcrossDevice:     true,
		EventPoolSize:               100,
		WelcomeMessage:              "欢迎使用{{appName}}",
		RootDir:                     "tsdddata",
		AdminPwd:                    "",
		// ---------- 外网配置 ----------
		External: struct {
			IP          string
			BaseURL     string
			H5BaseURL   string
			APIBaseURL  string
			WebLoginURL string
		}{
			BaseURL:     "",
			WebLoginURL: "",
		},

		// ---------- db配置 ----------
		DB: struct {
			MySQLAddr                  string
			MySQLMaxOpenConns          int
			MySQLMaxIdleConns          int
			MySQLConnMaxLifetime       time.Duration
			Migration                  bool
			RedisAddr                  string
			RedisPass                  string
			RedisTLS                   bool
			RedisTLSInsecureSkipVerify bool
			RedisTLSCAFile             string
			AsynctaskRedisAddr         string
		}{
			MySQLAddr:            "root:demo@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=true",
			MySQLMaxOpenConns:    100,
			MySQLMaxIdleConns:    10,
			MySQLConnMaxLifetime: time.Second * 60 * 60 * 4, //mysql 默认超时时间为 60*60*8=28800 SetConnMaxLifetime设置为小于数据库超时时间即可
			Migration:            true,
			RedisAddr:            "127.0.0.1:6379",
		},
		// ---------- 分布式配置 ----------
		Cluster: struct {
			NodeID int
		}{
			NodeID: 1,
		},
		// ---------- 缓存配置 ----------
		Cache: struct {
			TokenCachePrefix            string
			LoginDeviceCachePrefix      string
			LoginDeviceCacheExpire      time.Duration
			UIDTokenCachePrefix         string
			FriendApplyTokenCachePrefix string
			FriendApplyExpire           time.Duration
			TokenExpire                 time.Duration
			NameCacheExpire             time.Duration
		}{
			TokenCachePrefix:            "token:",
			TokenExpire:                 time.Hour * 24 * 30,
			LoginDeviceCachePrefix:      "login_device:",
			LoginDeviceCacheExpire:      time.Minute * 5,
			UIDTokenCachePrefix:         "uidtoken:",
			FriendApplyTokenCachePrefix: "friend_token:",
			FriendApplyExpire:           time.Hour * 24 * 15,
			NameCacheExpire:             time.Hour * 24 * 7,
		},

		// ---------- 系统账户设置 ----------
		Account: struct {
			SystemUID       string
			FileHelperUID   string
			SystemGroupID   string
			SystemGroupName string
			AdminUID        string
		}{
			SystemUID:       "u_10000",
			SystemGroupID:   "g_10000",
			SystemGroupName: "意见反馈群",
			FileHelperUID:   "fileHelper",
			AdminUID:        "admin",
		},
		// ---------- 文件服务 ----------
		FileService: FileServiceMinio,

		// ---------- 短信服务 ----------
		SMSProvider: SMSProviderAliyun,

		// ---------- wukongim ----------
		WuKongIM: struct {
			APIURL       string
			ManagerToken string
		}{
			APIURL: "http://127.0.0.1:5001",
		},

		// ---------- avatar ----------
		Avatar: struct {
			Default        string
			DefaultCount   int
			Partition      int
			DefaultBaseURL string
		}{
			// Default:        "assets/assets/avatar.png",
			Default:        "",
			DefaultCount:   900,
			Partition:      100,
			DefaultBaseURL: "",
		},

		// ---------- 短号配置 ----------
		ShortNo: struct {
			NumOn   bool
			NumLen  int
			EditOff bool
		}{
			NumOn:   false,
			NumLen:  7,
			EditOff: false,
		},

		// ---------- 机器人 ----------
		Robot: struct {
			MessageExpire      time.Duration
			InlineQueryTimeout time.Duration
			EventPoolSize      int64
		}{
			MessageExpire:      time.Hour * 24 * 7,
			InlineQueryTimeout: time.Second * 10,
			EventPoolSize:      100,
		},
		// ---------- gitee ----------
		Gitee: struct {
			OAuthURL     string
			ClientID     string
			ClientSecret string
		}{
			OAuthURL: "https://gitee.com/oauth/authorize",
		},
		// ---------- github ----------
		Github: struct {
			OAuthURL     string
			ClientID     string
			ClientSecret string
		}{
			OAuthURL: "https://github.com/login/oauth/authorize",
		},
		// ---------- rtc owt  ----------
		OWT: struct {
			URL          string
			ServiceID    string
			ServiceKey   string
			RoomMaxCount int
		}{
			RoomMaxCount: 9,
		},
		// ---------- livekit  ----------
		Livekit: struct {
			Off                  bool
			URL                  string
			APIKey               string
			APISecret            string
			InviteTimeoutSeconds int
			RoomMaxCount         int
		}{
			Off:                  false,
			InviteTimeoutSeconds: 60,
			RoomMaxCount:         9,
		},
		// ---------- 好友设置  --------

		Friend: struct {
			AddedTipsText string
		}{
			AddedTipsText: "我们已经是好友了，可以愉快的聊天了！",
		},
		// ---------- 群设置  ----------
		Group: struct {
			SameDayCreateMaxCount     int
			CreateGroupVerifyFriendOn bool
		}{
			SameDayCreateMaxCount:     10,
			CreateGroupVerifyFriendOn: true,
		},
		// ---------- push  ----------
		Push: struct {
			ContentDetailOn bool
			PushPoolSize    int64
			APNS            APNSPush
			MI              MIPush
			HMS             HMSPush
			VIVO            VIVOPush
			OPPO            OPPOPush
			FIREBASE        FIREBASEPush
		}{
			ContentDetailOn: true,
			PushPoolSize:    100,
			APNS: APNSPush{
				Dev:      true,
				Topic:    "com.xinbida.tangsengdaodao",
				Password: "123456",
			},
		},

		// ---------- support  ----------
		Support: struct {
			Email     string
			EmailSmtp string
			EmailPwd  string
		}{
			Email:     "",
			EmailSmtp: "smtp.exmail.qq.com:25",
			EmailPwd:  "",
		},

		QRCodeInfoURL: "v1/qrcode/:code",

		Test: GetEnvBool("Test", false),

		VisitorUIDPrefix: "_vt_",

		TimingWheelTick: duration{
			Duration: time.Millisecond * 10,
		},
		TimingWheelSize:      100,
		TablePartitionConfig: newTablePartitionConfig(),
		ElasticsearchURL:     "http://elasticsearch:9200",
	}
	// 检索读侧默认走 ZincSearch（现状）；ES 路线由部署显式切换。
	cfg.Search.ReadBackend = "zinc"
	// Kafka / ESIndex 默认关闭、地址留空，由部署仓注入（octo-lib 不内置生产端点）。
	cfg.Kafka.Topic = "octo.message.v1"
	cfg.Kafka.DLQTopic = "octo.message.v1.dlq"
	cfg.ESIndex.Index = "octo-message"

	return cfg
}

func (c *Config) ConfigureWithViper(vp *viper.Viper) {
	c.vp = vp
	intranetIP := getIntranetIP() // 内网IP
	// #################### 基础配置 ####################
	c.Mode = Mode(c.getString("mode", string(DebugMode)))
	c.AppID = c.getString("appID", c.AppID)
	c.AppName = c.getString("appName", c.AppName)
	c.RootDir = c.getString("rootDir", c.RootDir)
	c.Version = c.getString("version", c.Version)
	c.Addr = c.getString("addr", c.Addr)
	c.GRPCAddr = c.getString("grpcAddr", c.GRPCAddr)
	c.SSLAddr = c.getString("sslAddr", c.SSLAddr)
	c.MessageSaveAcrossDevice = c.getBool("messageSaveAcrossDevice", c.MessageSaveAcrossDevice)
	c.WelcomeMessage = c.getString("welcomeMessage", c.WelcomeMessage)
	if strings.TrimSpace(c.WelcomeMessage) != "" {
		c.WelcomeMessage = strings.ReplaceAll(c.WelcomeMessage, "{{appName}}", c.AppName)
	}
	c.PhoneSearchOff = c.getBool("phoneSearchOff", c.PhoneSearchOff)
	c.OnlineStatusOn = c.getBool("onlineStatusOn", c.OnlineStatusOn)
	c.GroupUpgradeWhenMemberCount = c.getInt("groupUpgradeWhenMemberCount", c.GroupUpgradeWhenMemberCount)
	c.EventPoolSize = c.getInt64("eventPoolSize", c.EventPoolSize)
	c.AdminPwd = c.getString("adminPwd", c.AdminPwd)

	// #################### 外网配置 ####################
	c.External.IP = c.getString("external.ip", c.External.IP)
	if strings.TrimSpace(c.External.IP) == "" { // 没配置外网IP就使用内网IP
		c.External.IP = intranetIP
	}
	c.External.WebLoginURL = c.getString("external.webLoginURL", c.External.WebLoginURL)
	c.External.BaseURL = c.getString("external.baseURL", c.External.BaseURL)

	if strings.TrimSpace(c.External.WebLoginURL) == "" {
		c.External.WebLoginURL = fmt.Sprintf("http://%s:82", c.External.IP)
	}

	if strings.TrimSpace(c.External.BaseURL) == "" {
		c.External.BaseURL = fmt.Sprintf("http://%s:8090", c.External.IP)
	}
	if strings.TrimSpace(c.External.H5BaseURL) == "" {
		c.External.H5BaseURL = fmt.Sprintf("%s/web", c.External.BaseURL)
	}
	if strings.TrimSpace(c.External.APIBaseURL) == "" {
		c.External.APIBaseURL = fmt.Sprintf("%s/v1", c.External.BaseURL)
	}
	// #################### 配置日志 ####################
	c.configureLog()
	// #################### db ####################
	c.DB.MySQLAddr = c.getString("db.mysqlAddr", c.DB.MySQLAddr)
	c.DB.MySQLMaxOpenConns = c.getInt("db.mysqlMaxOpenConns", c.DB.MySQLMaxOpenConns)
	c.DB.MySQLMaxIdleConns = c.getInt("db.mysqlMaxIdleConns", c.DB.MySQLMaxIdleConns)
	c.DB.MySQLConnMaxLifetime = c.getDuration("db.mysqlConnMaxLifetime", c.DB.MySQLConnMaxLifetime)
	c.DB.Migration = c.getBool("db.migration", c.DB.Migration)
	c.DB.RedisAddr = c.getString("db.redisAddr", c.DB.RedisAddr)
	c.DB.RedisPass = c.getString("db.redisPass", c.DB.RedisPass)
	c.DB.RedisTLS = c.getBool("db.redisTLS", c.DB.RedisTLS)
	c.DB.RedisTLSInsecureSkipVerify = c.getBool("db.redisTLSInsecureSkipVerify", c.DB.RedisTLSInsecureSkipVerify)
	c.DB.RedisTLSCAFile = c.getString("db.redisTLSCAFile", c.DB.RedisTLSCAFile)
	c.DB.AsynctaskRedisAddr = c.getString("db.asynctaskRedisAddr", c.DB.AsynctaskRedisAddr)

	//#################### cluster ####################
	c.Cluster.NodeID = c.getInt("cluster.nodeID", c.Cluster.NodeID)

	//#################### 缓存配置 ####################
	c.Cache.TokenCachePrefix = c.getString("cache.tokenCachePrefix", c.Cache.TokenCachePrefix)
	c.Cache.LoginDeviceCachePrefix = c.getString("cache.loginDeviceCachePrefix", c.Cache.LoginDeviceCachePrefix)
	c.Cache.LoginDeviceCacheExpire = c.getDuration("cache.loginDeviceCacheExpire", c.Cache.LoginDeviceCacheExpire)
	c.Cache.UIDTokenCachePrefix = c.getString("cache.uidTokenCachePrefix", c.Cache.UIDTokenCachePrefix)
	c.Cache.FriendApplyTokenCachePrefix = c.getString("cache.friendApplyTokenCachePrefix", c.Cache.FriendApplyTokenCachePrefix)
	c.Cache.FriendApplyExpire = c.getDuration("cache.friendApplyExpire", c.Cache.FriendApplyExpire)
	c.Cache.TokenExpire = c.getDuration("cache.tokenExpire", c.Cache.TokenExpire)
	c.Cache.NameCacheExpire = c.getDuration("cache.nameCacheExpire", c.Cache.NameCacheExpire)

	//#################### 内置账户配置 ####################
	c.Account.SystemUID = c.getString("account.systemUID", c.Account.SystemUID)
	c.Account.FileHelperUID = c.getString("account.fileHelperUID", c.Account.FileHelperUID)
	c.Account.SystemGroupID = c.getString("account.systemGroupID", c.Account.SystemGroupID)
	c.Account.SystemGroupName = c.getString("account.systemGroupName", c.Account.SystemGroupName)
	c.Account.AdminUID = c.getString("account.adminUID", c.Account.AdminUID)

	//#################### 文件服务 ####################
	c.FileService = FileService(c.getString("fileService", c.FileService.String()))
	// aliyun oss
	c.OSS.Endpoint = c.getString("oss.endpoint", c.OSS.Endpoint)
	c.OSS.BucketURL = c.getString("oss.bucketURL", c.OSS.BucketURL)
	c.OSS.AccessKeyID = c.getString("oss.accessKeyID", c.OSS.AccessKeyID)
	c.OSS.AccessKeySecret = c.getString("oss.accessKeySecret", c.OSS.AccessKeySecret)
	c.OSS.BucketName = c.getString("oss.bucketName", c.OSS.BucketName)
	// minio
	c.Minio.URL = c.getString("minio.url", c.Minio.URL)
	if c.FileService == FileServiceMinio {
		if strings.TrimSpace(c.Minio.URL) == "" {
			c.Minio.URL = fmt.Sprintf("http://%s:9000", c.External.IP)
		}
	}
	c.Minio.UploadURL = c.getString("minio.uploadURL", c.Minio.UploadURL)
	if strings.TrimSpace(c.Minio.UploadURL) == "" {
		c.Minio.UploadURL = c.Minio.URL
	}
	c.Minio.DownloadURL = c.getString("minio.downloadURL", c.Minio.DownloadURL)
	if strings.TrimSpace(c.Minio.DownloadURL) == "" {
		c.Minio.DownloadURL = c.Minio.URL
	}

	c.Minio.AccessKeyID = c.getString("minio.accessKeyID", c.Minio.AccessKeyID)
	c.Minio.SecretAccessKey = c.getString("minio.secretAccessKey", c.Minio.SecretAccessKey)
	// seaweedfs
	c.Seaweed.URL = c.getString("seaweed.url", c.Seaweed.URL)
	// qiniu 七牛云
	c.Qiniu.URL = c.getString("qiniu.url", c.Qiniu.URL)
	c.Qiniu.BucketName = c.getString("qiniu.bucketName", c.Qiniu.BucketName)
	c.Qiniu.AccessKey = c.getString("qiniu.accessKey", c.Qiniu.AccessKey)
	c.Qiniu.SecretKey = c.getString("qiniu.secretKey", c.Qiniu.SecretKey)
	// 腾讯云COS
	c.COS.SecretID = c.getString("cos.secretID", c.COS.SecretID)
	c.COS.SecretKey = c.getString("cos.secretKey", c.COS.SecretKey)
	c.COS.Bucket = c.getString("cos.bucket", c.COS.Bucket)
	c.COS.Region = c.getString("cos.region", c.COS.Region)
	c.COS.BucketURL = c.getString("cos.bucketURL", c.COS.BucketURL)
	c.COS.Prefix = c.getString("cos.prefix", c.COS.Prefix)
	// 环境变量覆盖（安全：避免密钥硬编码在配置文件中）
	StringEnv(&c.COS.SecretID, "TS_COS_SECRET_ID")
	StringEnv(&c.COS.SecretKey, "TS_COS_SECRET_KEY")
	// S3 / S3兼容存储
	c.S3.Endpoint = c.getString("s3.endpoint", c.S3.Endpoint)
	c.S3.Region = c.getString("s3.region", c.S3.Region)
	c.S3.Bucket = c.getString("s3.bucket", c.S3.Bucket)
	c.S3.AccessKeyID = c.getString("s3.accessKeyID", c.S3.AccessKeyID)
	c.S3.SecretAccessKey = c.getString("s3.secretAccessKey", c.S3.SecretAccessKey)
	c.S3.SessionToken = c.getString("s3.sessionToken", c.S3.SessionToken)
	c.S3.DownloadURL = c.getString("s3.downloadURL", c.S3.DownloadURL)
	c.S3.Prefix = c.getString("s3.prefix", c.S3.Prefix)
	c.S3.UsePathStyle = c.getBool("s3.usePathStyle", c.S3.UsePathStyle)
	// 环境变量覆盖（安全：避免长期凭据硬编码在配置文件中）
	// 优先级遵循"越具体越优先"：YAML → AWS 通用变量 → TS_ 项目专属变量。
	// AWS_* 提供 IRSA / IAM role 工作流的零配置兜底；TS_S3_* 表达对本应用的显式意图，
	// 最后写入故胜出，避免宿主机上无关的 AWS 凭据静默覆盖到 R2 / B2 / MinIO 等非 AWS 部署。
	StringEnv(&c.S3.AccessKeyID, "AWS_ACCESS_KEY_ID")
	StringEnv(&c.S3.SecretAccessKey, "AWS_SECRET_ACCESS_KEY")
	StringEnv(&c.S3.SessionToken, "AWS_SESSION_TOKEN")
	StringEnv(&c.S3.AccessKeyID, "TS_S3_ACCESS_KEY_ID")
	StringEnv(&c.S3.SecretAccessKey, "TS_S3_SECRET_ACCESS_KEY")
	StringEnv(&c.S3.SessionToken, "TS_S3_SESSION_TOKEN")

	//#################### 短信服务 ####################
	c.SMSCode = c.getString("smsCode", c.SMSCode)
	c.SMSProvider = SMSProvider(c.getString("smsProvider", string(c.SMSProvider)))
	// UniSMS
	c.UniSMS.AccessKeyID = c.getString("uniSMS.accessKeyID", c.UniSMS.AccessKeyID)
	c.UniSMS.Signature = c.getString("uniSMS.signature", c.UniSMS.Signature)
	c.UniSMS.AccessKeySecret = c.getString("uniSMS.accessKeySecret", c.UniSMS.AccessKeySecret)
	c.UniSMS.TemplateId = c.getString("uniSMS.templateId", c.UniSMS.TemplateId)

	//smsbao
	c.Smsbao.Account = c.getString("smsbao.account", c.Smsbao.Account)
	c.Smsbao.APIKey = c.getString("smsbao.apiKey", c.Smsbao.APIKey)
	c.Smsbao.Template = c.getString("smsbao.template", c.Smsbao.Template)
	// AliyunSMS
	c.AliyunSMS.AccessKeyID = c.getString("aliyunSMS.accessKeyID", c.AliyunSMS.AccessKeyID)
	c.AliyunSMS.AccessSecret = c.getString("aliyunSMS.accessSecret", c.AliyunSMS.AccessSecret)
	c.AliyunSMS.TemplateCode = c.getString("aliyunSMS.templateCode", c.AliyunSMS.TemplateCode)
	c.AliyunSMS.SignName = c.getString("aliyunSMS.signName", c.AliyunSMS.SignName)
	// AliyunInternationalSMS
	c.AliyunInternationalSMS.AccessKeyID = c.getString("aliyunInternationalSMS.accessKeyID", c.AliyunInternationalSMS.AccessKeyID)
	c.AliyunInternationalSMS.AccessSecret = c.getString("aliyunInternationalSMS.accessSecret", c.AliyunInternationalSMS.AccessSecret)
	c.AliyunInternationalSMS.SignName = c.getString("aliyunInternationalSMS.signName", c.AliyunInternationalSMS.SignName)

	//#################### 悟空IM ####################
	c.WuKongIM.APIURL = c.getString("wukongIM.apiURL", c.WuKongIM.APIURL)
	c.WuKongIM.ManagerToken = c.getString("wukongIM.managerToken", c.WuKongIM.ManagerToken)

	//#################### 头像 ####################
	c.Avatar.Default = c.getString("avatar.default", c.Avatar.Default)
	c.Avatar.DefaultCount = c.getInt("avatar.defaultCount", c.Avatar.DefaultCount)
	c.Avatar.Partition = c.getInt("avatar.partition", c.Avatar.Partition)
	c.Avatar.DefaultBaseURL = c.getString("avatar.defaultBaseURL", c.Avatar.DefaultBaseURL)

	//#################### 短号配置 ####################
	c.ShortNo.NumOn = c.getBool("shortNo.numOn", c.ShortNo.NumOn)
	c.ShortNo.NumLen = c.getInt("shortNo.numLen", c.ShortNo.NumLen)
	c.ShortNo.EditOff = c.getBool("shortNo.editOff", c.ShortNo.EditOff)

	//#################### 机器人 ####################
	c.Robot.MessageExpire = c.getDuration("robot.messageExpire", c.Robot.MessageExpire)
	c.Robot.InlineQueryTimeout = c.getDuration("robot.inlineQueryTimeout", c.Robot.InlineQueryTimeout)
	c.Robot.EventPoolSize = c.getInt64("robot.eventPoolSize", c.Robot.EventPoolSize)

	//#################### 第三方登录 ####################
	// gitee
	c.Gitee.OAuthURL = c.getString("gitee.oauthURL", c.Gitee.OAuthURL)
	c.Gitee.ClientID = c.getString("gitee.clientID", c.Gitee.ClientID)
	c.Gitee.ClientSecret = c.getString("gitee.clientSecret", c.Gitee.ClientSecret)
	// github
	c.Github.OAuthURL = c.getString("github.oauthURL", c.Github.OAuthURL)
	c.Github.ClientID = c.getString("github.clientID", c.Github.ClientID)
	c.Github.ClientSecret = c.getString("github.clientSecret", c.Github.ClientSecret)

	//#################### rtc owt ####################
	c.OWT.URL = c.getString("owt.url", c.OWT.URL)
	c.OWT.ServiceID = c.getString("owt.serviceID", c.OWT.ServiceID)
	c.OWT.ServiceKey = c.getString("owt.serviceKey", c.OWT.ServiceKey)
	c.OWT.RoomMaxCount = c.getInt("owt.roomMaxCount", c.OWT.RoomMaxCount)
	// #################### livekit ##########
	c.Livekit.Off = c.getBool("livekit.off", c.Livekit.Off)
	c.Livekit.URL = c.getString("livekit.url", c.Livekit.URL)
	c.Livekit.APIKey = c.getString("livekit.apiKey", c.Livekit.APIKey)
	c.Livekit.APISecret = c.getString("livekit.apiSecret", c.Livekit.APISecret)
	c.Livekit.RoomMaxCount = c.getInt("livekit.roomMaxCount", c.Livekit.RoomMaxCount)
	c.Livekit.InviteTimeoutSeconds = c.getInt("livekit.inviteTimeoutSeconds", c.Livekit.InviteTimeoutSeconds)
	//#################### register ####################
	c.Register.Off = c.getBool("register.off", c.Register.Off)
	c.Register.OnlyChina = c.getBool("register.onlyChina", c.Register.OnlyChina)
	c.Register.StickerAddOff = c.getBool("register.stickerAddOff", c.Register.StickerAddOff)
	c.Register.UsernameOn = c.getBool("register.usernameOn", c.Register.UsernameOn)
	c.Register.EmailOn = c.getBool("register.emailOn", c.Register.EmailOn)
	//#################### organization ####################
	c.Organization.ImportOn = c.getBool("organization.importOn", c.Organization.ImportOn)
	//#################### 好友 ###############
	c.Friend.AddedTipsText = c.getString("friend.addedTipsText", c.Friend.AddedTipsText)
	//#################### ZincSearch ####################
	c.ZincSearch.SearchOn = c.getBool("zincSearch.searchOn", c.ZincSearch.SearchOn)
	c.ZincSearch.APIURL = c.getString("zincSearch.apiURL", c.ZincSearch.APIURL)
	c.ZincSearch.ZincUsername = c.getString("zincSearch.username", c.ZincSearch.ZincUsername)
	c.ZincSearch.ZincPassword = c.getString("zincSearch.password", c.ZincSearch.ZincPassword)
	c.ZincSearch.SyncIntervalSecond = c.getInt("zincSearch.syncIntervalSecond", c.ZincSearch.SyncIntervalSecond)
	c.ZincSearch.SyncCount = c.getInt("zincSearch.syncCount", c.ZincSearch.SyncCount)
	//#################### 检索读侧后端切换 ####################
	c.Search.ReadBackend = c.getString("search.readBackend", c.Search.ReadBackend)
	//#################### Kafka 共享总线 ####################
	c.Kafka.On = c.getBool("kafka.on", c.Kafka.On)
	c.Kafka.Brokers = c.getStringSlice("kafka.brokers", c.Kafka.Brokers)
	c.Kafka.Topic = c.getString("kafka.topic", c.Kafka.Topic)
	c.Kafka.DLQTopic = c.getString("kafka.dlqTopic", c.Kafka.DLQTopic)
	//#################### es-indexer OpenSearch 写入 ####################
	c.ESIndex.On = c.getBool("esIndex.on", c.ESIndex.On)
	c.ESIndex.Addrs = c.getStringSlice("esIndex.addrs", c.ESIndex.Addrs)
	c.ESIndex.Username = c.getString("esIndex.username", c.ESIndex.Username)
	c.ESIndex.Password = c.getString("esIndex.password", c.ESIndex.Password)
	c.ESIndex.Index = c.getString("esIndex.index", c.ESIndex.Index)
	//#################### 群 #################
	c.Group.SameDayCreateMaxCount = c.getInt("group.sameDayCreateMaxCount", c.Group.SameDayCreateMaxCount)
	c.Group.CreateGroupVerifyFriendOn = c.getBool("group.createGroupVerifyFriendOn", c.Group.CreateGroupVerifyFriendOn)
	//#################### push ####################
	c.Push.ContentDetailOn = c.getBool("push.contentDetailOn", c.Push.ContentDetailOn)
	c.Push.PushPoolSize = c.getInt64("push.pushPoolSize", c.Push.PushPoolSize)
	// apns
	c.Push.APNS.Dev = c.getBool("push.apns.dev", c.Push.APNS.Dev)
	c.Push.APNS.Topic = c.getString("push.apns.topic", c.Push.APNS.Topic)
	c.Push.APNS.Password = c.getString("push.apns.password", c.Push.APNS.Password)
	c.Push.APNS.Cert = c.getString("push.apns.cert", c.Push.APNS.Cert)
	// 华为推送
	c.Push.HMS.PackageName = c.getString("push.hms.packageName", c.Push.HMS.PackageName)
	c.Push.HMS.AppID = c.getString("push.hms.appID", c.Push.HMS.AppID)
	c.Push.HMS.AppSecret = c.getString("push.hms.appSecret", c.Push.HMS.AppSecret)
	// 小米推送
	c.Push.MI.PackageName = c.getString("push.mi.packageName", c.Push.MI.PackageName)
	c.Push.MI.AppID = c.getString("push.mi.appID", c.Push.MI.AppID)
	c.Push.MI.AppSecret = c.getString("push.mi.appSecret", c.Push.MI.AppSecret)
	c.Push.MI.ChannelID = c.getString("push.mi.channelID", c.Push.MI.ChannelID)
	// vivo推送
	c.Push.VIVO.PackageName = c.getString("push.vivo.packageName", c.Push.VIVO.PackageName)
	c.Push.VIVO.AppID = c.getString("push.vivo.appID", c.Push.VIVO.AppID)
	c.Push.VIVO.AppKey = c.getString("push.vivo.appKey", c.Push.VIVO.AppKey)
	c.Push.VIVO.AppSecret = c.getString("push.vivo.appSecret", c.Push.VIVO.AppSecret)
	// oppo推送
	c.Push.OPPO.PackageName = c.getString("push.oppo.packageName", c.Push.OPPO.PackageName)
	c.Push.OPPO.AppID = c.getString("push.oppo.appID", c.Push.OPPO.AppID)
	c.Push.OPPO.AppKey = c.getString("push.oppo.appKey", c.Push.OPPO.AppKey)
	c.Push.OPPO.AppSecret = c.getString("push.oppo.appSecret", c.Push.OPPO.AppSecret)
	c.Push.OPPO.MasterSecret = c.getString("push.oppo.masterSecret", c.Push.OPPO.MasterSecret)
	// FIREBASE 推送
	c.Push.FIREBASE.JsonPath = c.getString("push.firebase.jsonPath", c.Push.FIREBASE.JsonPath)
	c.Push.FIREBASE.ProjectId = c.getString("push.firebase.projectId", c.Push.FIREBASE.ProjectId)
	c.Push.FIREBASE.PackageName = c.getString("push.firebase.packageName", c.Push.FIREBASE.PackageName)
	//#################### message ####################
	c.Message.SendMessageOn = c.getBool("message.sendMessageOn", c.Message.SendMessageOn)
	c.Message.SyncReadedCountIntervalSecond = c.getInt("message.syncReadedCountIntervalSecond", c.Message.SyncReadedCountIntervalSecond)
	//#################### weixin ####################
	c.Wechat.AppID = c.getString("wechat.appID", c.Wechat.AppID)
	c.Wechat.AppSecret = c.getString("wechat.appSecret", c.Wechat.AppSecret)

	//#################### tracing ####################
	c.Tracing.On = c.getBool("tracing.on", c.Tracing.On)
	c.Tracing.Addr = c.getString("tracing.addr", c.Tracing.Addr)

	//#################### support ####################
	c.Support.Email = c.getString("support.email", c.Support.Email)
	c.Support.EmailSmtp = c.getString("support.emailSmtp", c.Support.EmailSmtp)
	c.Support.EmailPwd = c.getString("support.emailPwd", c.Support.EmailPwd)

}

func (c *Config) ConfigFileUsed() string {
	return c.vp.ConfigFileUsed()
}

func (c *Config) configureLog() {
	logLevel := c.vp.GetInt("logger.level")
	// level
	if logLevel == 0 { // 没有设置
		if c.Mode == DebugMode {
			logLevel = int(zapcore.DebugLevel)
		} else {
			logLevel = int(zapcore.InfoLevel)
		}
	} else {
		logLevel = logLevel - 2
	}
	c.Logger.Level = zapcore.Level(logLevel)
	c.Logger.Dir = c.vp.GetString("logger.dir")
	if strings.TrimSpace(c.Logger.Dir) == "" {
		c.Logger.Dir = "logs"
	}
	if !strings.HasPrefix(strings.TrimSpace(c.Logger.Dir), "/") {
		c.Logger.Dir = filepath.Join(c.RootDir, c.Logger.Dir)
	}
	c.Logger.LineNum = c.vp.GetBool("logger.lineNum")
}

func (c *Config) getString(key string, defaultValue string) string {
	v := c.vp.GetString(key)
	if v == "" {
		return defaultValue
	}
	return v
}

// getStringSlice 读取字符串列表配置。支持两种形态：YAML/JSON 数组（["a","b"]），以及
// 单标量逗号分隔字符串（"a,b" —— env/简写常用）。viper.GetStringSlice 对逗号标量只按
// 空白切分，不拆逗号，故这里先取标量再按逗号显式拆分兜底。为空（未配置）返回默认值。
func (c *Config) getStringSlice(key string, defaultValue []string) []string {
	// 优先取原始标量，命中含逗号的简写形态时显式拆分。
	if raw := strings.TrimSpace(c.vp.GetString(key)); raw != "" && strings.Contains(raw, ",") {
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	v := c.vp.GetStringSlice(key)
	if len(v) == 0 {
		return defaultValue
	}
	return v
}

func (c *Config) getBool(key string, defaultValue bool) bool {
	objV := c.vp.Get(key)
	if objV == nil {
		return defaultValue
	}
	return cast.ToBool(objV)
}
func (c *Config) getInt(key string, defaultValue int) int {
	v := c.vp.GetInt(key)
	if v == 0 {
		return defaultValue
	}
	return v
}

func (c *Config) getInt64(key string, defaultValue int64) int64 {
	v := c.vp.GetInt64(key)
	if v == 0 {
		return defaultValue
	}
	return v
}

func (c *Config) getDuration(key string, defaultValue time.Duration) time.Duration {
	v := c.vp.GetDuration(key)
	if v == 0 {
		return defaultValue
	}
	return v
}

// GetAvatarPath 获取用户头像path
func (c *Config) GetAvatarPath(uid string) string {
	return fmt.Sprintf("users/%s/avatar", uid)
}

// avatarVersion 从变参中取头像版本号。
// 仅取第一个参数(first-wins),其余忽略;缺省或非正值表示使用旧版(无版本)路径,
// 从而保持向后兼容。
func avatarVersion(version []int64) int64 {
	if len(version) > 0 {
		return version[0]
	}
	return 0
}

// GetGroupAvatarFilePath 获取群头像上传路径。
// 不传 version 或 version <= 0 时返回旧版稳定路径;version > 0 时返回带版本的路径,
// 使对象键随每次上传变化,以规避忽略 query string 的 CDN 缓存。
func (c *Config) GetGroupAvatarFilePath(groupNo string, version ...int64) string {
	avatarID := crc32.ChecksumIEEE([]byte(groupNo)) % uint32(c.Avatar.Partition)
	if v := avatarVersion(version); v > 0 {
		return fmt.Sprintf("group/%d/%s/%d.png", avatarID, groupNo, v)
	}
	return fmt.Sprintf("group/%d/%s.png", avatarID, groupNo)
}

// GetCommunityAvatarFilePath 获取社区头像上传路径。
// 不传 version 或 version <= 0 时返回旧版稳定路径;version > 0 时返回带版本的路径。
func (c *Config) GetCommunityAvatarFilePath(communityNo string, version ...int64) string {
	avatarID := crc32.ChecksumIEEE([]byte(communityNo)) % uint32(c.Avatar.Partition)
	if v := avatarVersion(version); v > 0 {
		return fmt.Sprintf("community/%d/%s/%d.png", avatarID, communityNo, v)
	}
	return fmt.Sprintf("community/%d/%s.png", avatarID, communityNo)
}

// GetCommunityCoverFilePath 获取社区封面上传路径。
// 不传 version 或 version <= 0 时返回旧版稳定路径;version > 0 时返回带版本的路径,
// 版本路径形态避免与旧版 community/<partition>/<community_no>_cover.png 冲突。
func (c *Config) GetCommunityCoverFilePath(communityNo string, version ...int64) string {
	avatarID := crc32.ChecksumIEEE([]byte(communityNo)) % uint32(c.Avatar.Partition)
	if v := avatarVersion(version); v > 0 {
		return fmt.Sprintf("community/%d/%s/cover/%d.png", avatarID, communityNo, v)
	}
	return fmt.Sprintf("community/%d/%s_cover.png", avatarID, communityNo)
}

// GetOrganizationLogoFilePath 获取组织logo上传路径
func (c *Config) GetOrganizationLogoFilePath(orgId string) string {
	return fmt.Sprintf("organization/logo/%s.png", orgId)
}

// IsVisitorChannel 是访客频道
func (c *Config) IsVisitorChannel(uid string) bool {

	return strings.HasSuffix(uid, "@ht")
}

// 获取客服频道真实ID
func (c *Config) GetCustomerServiceChannelID(channelID string) (string, bool) {
	if !strings.Contains(channelID, "|") {
		return "", false
	}
	channelIDs := strings.Split(channelID, "|")
	return channelIDs[1], true
}

// 获取客服频道的访客id
func (c *Config) GetCustomerServiceVisitorUID(channelID string) (string, bool) {
	if !strings.Contains(channelID, "|") {
		return "", false
	}
	channelIDs := strings.Split(channelID, "|")
	return channelIDs[0], true
}

// 组合客服ID
func (c *Config) ComposeCustomerServiceChannelID(vid string, channelID string) string {
	return fmt.Sprintf("%s|%s", vid, channelID)
}

// IsVisitor 是访客uid
func (c *Config) IsVisitor(uid string) bool {

	return strings.HasPrefix(uid, c.VisitorUIDPrefix)
}

// GetEnv 成环境变量里获取
func GetEnv(key string, defaultValue string) string {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return defaultValue
	}
	return v
}

// GetEnvBool 成环境变量里获取
func GetEnvBool(key string, defaultValue bool) bool {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return defaultValue
	}
	if v == "true" {
		return true
	}
	return false
}

// GetEnvInt64 环境变量获取
func GetEnvInt64(key string, defaultValue int64) int64 {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return defaultValue
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		fmt.Printf("WARN: invalid env %s=%q, using default %d: %v\n", key, v, defaultValue, err)
		return defaultValue
	}
	return i
}

// GetEnvInt 环境变量获取
func GetEnvInt(key string, defaultValue int) int {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return defaultValue
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		fmt.Printf("WARN: invalid env %s=%q, using default %d: %v\n", key, v, defaultValue, err)
		return defaultValue
	}
	return int(i)
}

// GetEnvFloat64 环境变量获取
func GetEnvFloat64(key string, defaultValue float64) float64 {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return defaultValue
	}
	i, err := strconv.ParseFloat(v, 64)
	if err != nil {
		fmt.Printf("WARN: invalid env %s=%q, using default %g: %v\n", key, v, defaultValue, err)
		return defaultValue
	}
	return i
}

// StringEnv StringEnv
func StringEnv(v *string, key string) {
	vv := os.Getenv(key)
	if vv != "" {
		*v = vv
	}
}

// BoolEnv 环境bool值
func BoolEnv(b *bool, key string) {
	value := os.Getenv(key)
	if strings.TrimSpace(value) != "" {
		if value == "true" {
			*b = true
		} else {
			*b = false
		}
	}
}

// 获取内网地址
func getIntranetIP() string {
	intranetIPs, err := util.GetIntranetIP()
	if err != nil {
		panic(err)
	}
	if len(intranetIPs) > 0 {
		return intranetIPs[0]
	}
	return ""
}

// SMSProvider 短信供应者
type SMSProvider string

const (
	// SMSProviderAliyun aliyun
	SMSProviderAliyun SMSProvider = "aliyun"
	SMSProviderUnisms SMSProvider = "unisms" // 联合短信(https://unisms.apistd.com/docs/api/send/)
	SMSProviderSmsbao SMSProvider = "smsbao" // 短信宝(https://www.smsbao.com/)
)

// AliyunSMSConfig 阿里云短信
type AliyunSMSConfig struct {
	AccessKeyID  string // aliyun的AccessKeyID
	AccessSecret string // aliyun的AccessSecret
	TemplateCode string // aliyun的短信模版
	SignName     string // 签名
}

// aliyun oss
type OSSConfig struct {
	Endpoint        string
	BucketName      string // Bucket名称 比如 tangsengdaodao
	BucketURL       string // 文件下载地址域名 对应aliyun的Bucket域名
	AccessKeyID     string
	AccessKeySecret string
}

type MinioConfig struct {
	URL             string // 文件下载上传或下载基地址 例如： http://127.0.0.1:9000
	UploadURL       string // 文件上传基地址 如果为空则使用URL地址
	DownloadURL     string // 文件下载基地址 如果为空则使用URL地址
	AccessKeyID     string //minio accessKeyID
	SecretAccessKey string //minio secretAccessKey
}

type SeaweedConfig struct {
	URL string // 文件下载上传基地址
}

type QiniuConfig struct {
	URL        string
	BucketName string
	AccessKey  string
	SecretKey  string
}

// S3Config configures any S3-compatible object storage backend.
// Use this for AWS S3 or third-party providers that speak the S3 protocol
// (Cloudflare R2, Backblaze B2, Wasabi, etc.). Tencent Cloud COS and MinIO
// continue to be configured via their existing dedicated blocks for
// backward compatibility.
type S3Config struct {
	// Endpoint is the S3 API hostname, without scheme.
	// Examples:
	//   AWS S3:        s3.us-west-2.amazonaws.com
	//   Cloudflare R2: <accountid>.r2.cloudflarestorage.com
	//   MinIO:         minio.example.com:9000
	// REQUIRED — no default.
	Endpoint string

	// Region is the SigV4 signing region.
	// Examples: us-west-2, ap-southeast-1, auto (R2).
	// REQUIRED — must match the bucket's actual region for AWS; some
	// providers accept any value (e.g. R2 uses "auto").
	Region string

	// Bucket is the single bucket all uploads target. Object keys carry
	// their own logical prefix (chat/, moment/, ...), so a single bucket
	// is the recommended deployment topology.
	Bucket string

	// AccessKeyID / SecretAccessKey are the long-lived IAM credentials
	// for the bucket. See the security docs for the minimum IAM policy.
	AccessKeyID     string
	SecretAccessKey string

	// SessionToken is the AWS STS session token for temporary credentials.
	// Empty for long-lived IAM user keys; populated by IRSA / IMDSv2 /
	// EKS Pod Identity / AWS SSO workflows that issue rotating credentials.
	//
	// Lifecycle: the service layer does NOT refresh the token itself — the
	// deployment pipeline is responsible for refreshing AWS_SESSION_TOKEN
	// (and the paired key/secret) before STS expiry. Leave empty for
	// static credentials.
	//
	// Override precedence: YAML s3.sessionToken → AWS_SESSION_TOKEN →
	// TS_S3_SESSION_TOKEN (project-prefixed wins). Same pattern as the
	// AccessKeyID / SecretAccessKey pair.
	SessionToken string

	// DownloadURL is the browser-facing base URL used for unsigned object
	// access (e.g. preview redirects, upload-response `path` field). It
	// does NOT participate in SigV4 signing — presigned PUT/GET URLs are
	// always signed against Endpoint.
	//
	// Use cases:
	//   - Empty: callers fall back to the canonical hostname constructed
	//     from Endpoint + Bucket.
	//   - CloudFront / CDN front-door: set to the distribution URL, e.g.
	//     https://d123.cloudfront.net. The browser fetches reads via
	//     CloudFront (typically with OAC for private buckets); presigned
	//     PUT/GET still flow directly to S3.
	//   - Custom domain: set to a bucket-subdomain custom domain, e.g.
	//     https://my-bucket.cdn.example.com.
	//
	// Must be scheme://host[:port] with no path component (same contract
	// as MinioConfig.DownloadURL).
	DownloadURL string

	// Prefix is an optional object-key prefix for multi-environment
	// isolation (e.g. "staging/" or "prod/"). Mirrors COSConfig.Prefix.
	Prefix string

	// UsePathStyle forces path-style addressing
	// (https://endpoint/bucket/key) instead of virtual-hosted style
	// (https://bucket.endpoint/key). Required by some S3 gateways and
	// by AWS for very old buckets or path-style-only endpoints; rarely
	// needed for AWS S3 in 2026, default false.
	UsePathStyle bool
}

// COSConfig 腾讯云COS配置
type COSConfig struct {
	SecretID  string // 腾讯云 SecretId
	SecretKey string // 腾讯云 SecretKey
	Bucket    string // 存储桶名（如 my-bucket-12345678）
	Region    string // 地域（如 ap-beijing）
	BucketURL string // 访问域名（如 https://my-bucket-12345678.cos.example.com）
	Prefix    string // 环境前缀，多环境共用 bucket 时隔离路径（如 "dev"）
}

// UnismsConfig unisms短信
type UnismsConfig struct {
	Signature       string
	AccessKeyID     string
	AccessKeySecret string
	TemplateId      string
}
type SmsbaoConfig struct {
	Account  string
	APIKey   string
	Template string
}

// AliyunInternationalSMSConfig 阿里云短信
type AliyunInternationalSMSConfig struct {
	AccessKeyID  string // aliyun的AccessKeyID
	AccessSecret string // aliyun的AccessSecret
	SignName     string // 签名
}

// 苹果推送
type APNSPush struct {
	Dev      bool
	Topic    string
	Password string
	Cert     string
}

// 华为推送
type HMSPush struct {
	PackageName string
	AppID       string
	AppSecret   string
}

// 小米推送
type MIPush struct {
	PackageName string
	AppID       string
	AppSecret   string
	ChannelID   string
}

// oppo推送
type OPPOPush struct {
	PackageName  string
	AppID        string
	AppKey       string
	AppSecret    string
	MasterSecret string
}

type VIVOPush struct {
	PackageName string
	AppID       string
	AppKey      string
	AppSecret   string
}

type FIREBASEPush struct {
	PackageName string
	JsonPath    string // firebase推送需要的json的路径
	ProjectId   string // serviceAccountJson中的project_id值
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

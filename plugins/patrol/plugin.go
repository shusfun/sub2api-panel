package patrolplugin

import "sub2api-panel/internal/pluginrt"

const (
	PatrolPluginID = "patrol-adaptive"
	GuestPluginID  = "guest-self-check"
)

func init() {
	pluginrt.Register(pluginrt.Manifest{
		ID:          PatrolPluginID,
		Name:        "流量门控智能渠道巡检",
		Description: "按滑动窗口内真实活跃用户数自适应唤醒分组巡检，闲时零 Token 消耗，支持将脱敏健康看板挂载到公开首页或开放给登录用户。",
		Category:    "observability",
		DefaultOn:   true,
		Settings: []pluginrt.Setting{
			{Key: "home_visible", Title: "在公开首页内嵌展示巡检看板", Description: "开启后访客在首页即可看到已勾选公开的分组健康状态。", Kind: "bool", Default: true},
			{Key: "user_visible", Title: "向登录普通用户开放渠道巡检页", Description: "开启后普通用户工作台导航增加只读巡检视图。", Kind: "bool", Default: true},
			{Key: "retention_days", Title: "巡检记录保留天数", Description: "到期自动按时间索引清理历史记录。", Kind: "int", Default: 30, Min: 1, Max: 180},
		},
		Apply: func(c *pluginrt.Ctx) error {
			c.Note("活跃门控调度已就绪")
			return nil
		},
	})

	pluginrt.Register(pluginrt.Manifest{
		ID:          GuestPluginID,
		Name:        "公开页访客自带 Key 自助检测",
		Description: "在公开巡检区提供一次性内存直发自助检测表单（消耗访客自己填写的密钥，服务端零落盘），默认关闭。",
		Category:    "public",
		DefaultOn:   false,
		Settings: []pluginrt.Setting{
			{Key: "default_model", Title: "默认推荐检测模型", Kind: "string", Default: "gpt-5.4"},
			{Key: "rpm_per_ip", Title: "单 IP 每分钟自助检测上限", Kind: "int", Default: 4, Min: 1, Max: 30},
		},
		Apply: func(c *pluginrt.Ctx) error {
			c.Note("公开访客自助检测已启用")
			return nil
		},
	})
}

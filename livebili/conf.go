package livebili

import (
	"github.com/kohmebot/plugin/v2/ui"
	"math/rand"
)

type Config struct {
	// 订阅的uid
	Uids []int64 `yaml:"uids" jsonschema:"description=订阅的uid"`

	// 指定分群订阅uid
	GroupUids map[int64][]int64 `yaml:"group_uids" jsonschema:"description=指定分群订阅uid"`

	// 不推送直播状态的uid
	NoLiveUids []int64 `yaml:"no_live_uids" jsonschema:"description=不推送直播状态的uid"`

	// 不推送粉丝数的uid
	NoFollowerUids []int64 `yaml:"no_follower_uids" jsonschema:"description=不推送粉丝数的uid"`

	// 不推送动态的uid
	NoDynamicUids []int64 `yaml:"no_dynamic_uids" jsonschema:"description=不推送动态的uid"`

	// 字体文件路径
	TTF string `yaml:"ttf" jsonschema:"description=字体文件路径"`

	// 检查直播间状态的间隔时间，单位为秒
	CheckLiveDuration int `yaml:"check_live_duration" jsonschema:"description=检查直播间状态的间隔时间，单位为秒"`

	// 直播开始时推送的提示语
	LiveTips []string `yaml:"live_tips" jsonschema:"description=直播开始时推送的提示语"`

	// 直播结束时推送的提示语
	OffTips []string `yaml:"off_tips" jsonschema:"description=直播结束时推送的提示语"`

	// 下播是否推送
	SendOff bool `yaml:"send_off" jsonschema:"description=下播是否推送"`

	// 检查动态的间隔时间，单位为秒
	CheckDynamicDuration int `yaml:"check_dynamic_duration" jsonschema:"description=检查动态的间隔时间，单位为秒"`

	// 检查粉丝数的间隔时间，单位为秒
	CheckFollowerDuration int `yaml:"check_follower_duration" jsonschema:"description=检查粉丝数的间隔时间，单位为秒"`

	// 粉丝数变化时，每多少个推送一次
	FollowerNotifyEach int `yaml:"follower_notify_each" jsonschema:"description=粉丝数变化时，每多少个推送一次"`

	// 当达到这个时间间隔时，推送一次粉丝数变化
	FollowerNotifyDuration int `yaml:"follower_notify_duration" jsonschema:"description=当达到这个时间间隔时，推送一次粉丝数变化"`

	// 每个uid检查的间隔时间，单位为秒
	CheckDuration int `yaml:"check_duration" jsonschema:"description=每个uid检查的间隔时间|单位为秒"`

	// b站的cookies
	Cookies ui.TextArea `yaml:"cookies" jsonschema:"description=b站的cookies"`

	// 是否at全员
	AtAll bool `yaml:"at_all" jsonschema:"description=是否at全员"`
}

func (c *Config) randChoseLiveTips() string {
	return c.LiveTips[rand.Intn(len(c.LiveTips))]
}
func (c *Config) randChoseOffTips() string {
	return c.OffTips[rand.Intn(len(c.OffTips))]
}

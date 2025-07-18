package livebili

import (
	"math/rand"
)

type Config struct {
	// 订阅的uid
	Uids []int64 `yaml:"uids" mapstructure:"uids"`
	// 指定分群订阅uid
	GroupUids map[int64][]int64 `yaml:"group_uids" mapstructure:"group_uids"`
	// 字体文件路径
	TTF string `yaml:"ttf" mapstructure:"ttf"`
	// 检查直播间状态的间隔时间，单位为秒
	CheckLiveDuration int `yaml:"check_live_duration" mapstructure:"check_live_duration"`
	// 直播开始时推送的提示语
	LiveTips []string `yaml:"live_tips" mapstructure:"live_tips"`
	// 直播结束时推送的提示语
	OffTips []string `yaml:"off_tips" mapstructure:"off_tips"`
	// 下播是否推送
	SendOff bool `yaml:"send_off" mapstructure:"send_off"`
	// 检查动态的间隔时间，单位为秒
	CheckDynamicDuration int `yaml:"check_dynamic_duration" mapstructure:"check_dynamic_duration"`
	// 检查粉丝数的间隔时间，单位为秒
	CheckFollowerDuration int `yaml:"check_follower_duration" mapstructure:"check_follower_duration"`
	// 粉丝数变化时，每多少个推送一次
	FollowerNotifyEach int `yaml:"follower_notify_each" mapstructure:"follower_notify_each"`
	// 当达到这个时间间隔时，推送一次粉丝数变化
	FollowerNotifyDuration int `yaml:"follower_notify_duration" mapstructure:"follower_notify_duration"`
	// b站的cookies
	Cookies string `yaml:"cookies" mapstructure:"cookies"`
	// 是否at全员
	AtAll bool `yaml:"at_all" mapstructure:"at_all"`
}

func (c *Config) randChoseLiveTips() string {
	return c.LiveTips[rand.Intn(len(c.LiveTips))]
}
func (c *Config) randChoseOffTips() string {
	return c.OffTips[rand.Intn(len(c.OffTips))]
}

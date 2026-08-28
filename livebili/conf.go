package livebili

import (
	"math/rand"
	"sort"
	"strings"

	"github.com/kohmebot/plugin/v2/ui"
)

// PushConfig 描述一个 B 站用户的全部推送策略。
type PushConfig struct {
	// 推送目标群。字段缺省时推送到插件所在的全部群；显式配置 [] 时不推送到任何群。
	Groups []int64 `yaml:"groups" jsonschema:"description=推送目标群|缺省时推送到全部群|配置为空数组时不推送"`

	// 是否推送粉丝数变化。
	SendFollower bool `yaml:"send_follower" jsonschema:"description=是否推送粉丝数变化"`

	// 是否推送直播状态变化。
	SendLive bool `yaml:"send_live" jsonschema:"description=是否推送直播状态变化"`

	// 是否推送动态。
	SendDynamic bool `yaml:"send_dynamic" jsonschema:"description=是否推送动态"`

	// 直播开始时随机选用的提示语。
	LiveTips []string `yaml:"live_tips" jsonschema:"description=直播开始时随机选用的提示语"`

	// 直播结束时随机选用的提示语。
	OffTips []string `yaml:"off_tips" jsonschema:"description=直播结束时随机选用的提示语"`

	// 直播和动态推送是否艾特全员；免打扰时仍不会艾特。
	AtAll bool `yaml:"at_all" jsonschema:"description=直播和动态推送是否艾特全员|免打扰时仍不会艾特"`

	// 下播时是否推送。仅在 send_live 为 true 时生效。
	SendOff bool `yaml:"send_off" jsonschema:"description=下播时是否推送|仅在开启直播状态推送时生效"`
}

type Config struct {
	// 以 B 站 UID 为键的推送配置。
	UIDs map[int64]PushConfig `yaml:"uids" jsonschema:"description=以B站UID为键的推送配置"`

	// 字体文件路径
	// Deprecated: HTML 卡片使用浏览器字体栈，此字段仅保留旧配置兼容。
	//TTF string `yaml:"ttf" jsonschema:"description=旧版绘图字体路径|HTML 卡片已不再使用"`

	// Chrome DevTools WebSocket 地址。留空时使用本机 Chrome。
	ChromeWs string `yaml:"chrome_ws" jsonschema:"description=Chrome DevTools WebSocket 地址|留空时使用本机 Chrome"`

	// 检查直播间状态的间隔时间，单位为秒。
	CheckLiveDuration int `yaml:"check_live_duration" jsonschema:"description=检查直播间状态的间隔时间|单位为秒"`

	// 检查动态的间隔时间，单位为秒。
	CheckDynamicDuration int `yaml:"check_dynamic_duration" jsonschema:"description=检查动态的间隔时间|单位为秒"`

	// 检查粉丝数的间隔时间，单位为秒。
	CheckFollowerDuration int `yaml:"check_follower_duration" jsonschema:"description=检查粉丝数的间隔时间|单位为秒"`

	// 粉丝数变化时，每多少个推送一次。
	FollowerNotifyEach int `yaml:"follower_notify_each" jsonschema:"description=粉丝数变化时每多少个推送一次"`

	// 当达到这个时间间隔时，推送一次粉丝数变化。
	FollowerNotifyDuration int `yaml:"follower_notify_duration" jsonschema:"description=粉丝数变化通知的最长间隔|单位为秒"`

	// 每个 UID 检查的间隔时间，单位为秒。
	CheckDuration int `yaml:"check_duration" jsonschema:"description=每个UID检查的间隔时间|单位为秒"`

	// B 站 Cookies。
	Cookies ui.TextArea `yaml:"cookies" jsonschema:"description=B站Cookies"`
}

func (c Config) ChromeAddr() string {
	addr := strings.TrimSpace(c.ChromeWs)
	if addr == "" || strings.HasPrefix(addr, "ws://") || strings.HasPrefix(addr, "wss://") {
		return addr
	}
	return "ws://" + addr
}

// enabledUIDs 返回启用了指定推送类型的 UID，并排序以保证检查顺序稳定。
func (c Config) enabledUIDs(enabled func(PushConfig) bool) []int64 {
	uids := make([]int64, 0, len(c.UIDs))
	for uid, push := range c.UIDs {
		if enabled(push) {
			uids = append(uids, uid)
		}
	}
	sort.Slice(uids, func(i, j int) bool { return uids[i] < uids[j] })
	return uids
}

func (c Config) allUIDs() []int64 {
	return c.enabledUIDs(func(PushConfig) bool { return true })
}

func (p PushConfig) randomLiveTip() string {
	return randomTip(p.LiveTips)
}

func (p PushConfig) randomOffTip() string {
	return randomTip(p.OffTips)
}

func randomTip(tips []string) string {
	if len(tips) == 0 {
		return ""
	}
	return tips[rand.Intn(len(tips))]
}

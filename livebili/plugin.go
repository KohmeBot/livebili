package livebili

import (
	"fmt"
	"github.com/kohmebot/gn8/gn8sdk"
	"github.com/kohmebot/pkg/command"
	"github.com/kohmebot/pkg/version"
	"github.com/kohmebot/plugin"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
)

type biliPlugin struct {
	e       *zero.Engine
	env     plugin.Env
	groups  plugin.Groups
	conf    Config
	gn8Iv   *gn8
	ttfPath string
}

func NewPlugin() plugin.Plugin {
	return &biliPlugin{}
}

func (b *biliPlugin) Init(engine *zero.Engine, env plugin.Env) error {
	b.e = engine
	b.env = env
	b.groups = env.Groups()
	i, err := gn8sdk.NewInvoker(env)
	if err != nil {
		logrus.Warnf("未找到gn8插件，将不开启免打扰功能")
	}
	b.gn8Iv = &gn8{i: i}
	return b.init()
}

func (b *biliPlugin) Name() string {
	return "livebili"
}

func (b *biliPlugin) Description() string {
	return "推送bilibili动态"
}

func (b *biliPlugin) Commands() fmt.Stringer {
	return command.NewCommands()
}

func (b *biliPlugin) Version() uint64 {
	return uint64(version.NewVersion(0, 0, 62))
}

func (b *biliPlugin) OnBoot() {

}

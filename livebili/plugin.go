package livebili

import (
	"github.com/kohmebot/gn8/gn8sdk"
	"github.com/kohmebot/plugin/v2"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
)

type biliPlugin struct {
	e       plugin.Engine
	env     plugin.Env
	groups  plugin.Groups
	conf    Config
	gn8Iv   *gn8
	ttfPath string
}

func NewPlugin() plugin.Plugin {
	return &biliPlugin{}
}

func (b *biliPlugin) OnInit(engine plugin.Engine, env plugin.Env) error {
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

func (b *biliPlugin) OnHelp(*zero.Ctx) {

}

func (b *biliPlugin) Name() string {
	return "livebili"
}

func (b *biliPlugin) Version() string {
	return "v0.1.11"
}

func (b *biliPlugin) OnBoot() {

}

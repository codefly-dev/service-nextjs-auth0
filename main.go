package main

import (
	"embed"

	"github.com/codefly-dev/cli/pkg/plugins"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	"github.com/codefly-dev/core/configurations"
	"github.com/codefly-dev/core/shared"
)

// Plugin version
var conf = configurations.LoadPluginConfiguration(shared.Embed(info))

type Settings struct {
	Debug bool `yaml:"debug"` // Developer only
	Watch bool `yaml:"watch"`
}

type Service struct {
	*services.Base

	// Settings
	*Settings
}

func NewService() *Service {
	return &Service{
		Base:     services.NewServiceBase(conf.Of(configurations.PluginService)),
		Settings: &Settings{},
	}
}

func main() {
	plugins.Register(
		services.NewFactoryPlugin(conf.Of(configurations.PluginFactoryService), NewFactory()),
		services.NewRuntimePlugin(conf.Of(configurations.PluginRuntimeService), NewRuntime()))
}

//go:embed plugin.codefly.yaml
var info embed.FS

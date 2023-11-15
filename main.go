package main

import (
	"embed"

	"github.com/codefly-dev/core/shared"

	"github.com/codefly-dev/cli/pkg/plugins"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	"github.com/codefly-dev/core/configurations"
)

// Plugin version
var plugin = configurations.LoadPluginConfiguration(shared.Embed(info))

type Settings struct {
	Debug              bool `yaml:"debug"` // Developer only
	Watch              bool `yaml:"watch"`
	WithDebugSymbols   bool `yaml:"with-debug-symbols"`
	CreateHttpEndpoint bool `yaml:"create-rest-endpoint"`
}

type Service struct {
	*services.Base

	// Settings
	*Settings
}

func NewService() *Service {
	return &Service{
		Base:     services.NewServiceBase(plugin.Of(configurations.PluginService)),
		Settings: &Settings{},
	}
}

func main() {
	plugins.Register(
		services.NewFactoryPlugin(plugin.Of(configurations.PluginFactoryService), NewFactory()),
		services.NewRuntimePlugin(plugin.Of(configurations.PluginRuntimeService), NewRuntime()))
}

//go:embed plugin.codefly.yaml
var info embed.FS

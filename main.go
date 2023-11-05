package main

import (
	"github.com/codefly-dev/cli/pkg/plugins"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	"github.com/codefly-dev/core/configurations"
)

// Plugin version
var conf = configurations.Plugin{
	Publisher:  "codefly.ai",
	Identifier: "nextjs",
	Kind:       configurations.PluginService,
	Version:    "0.0.0",
}

type Spec struct {
	Debug bool `yaml:"debug"` // Developer only
	Watch bool `yaml:"watch"`
}

type Service struct {
	*services.Base

	// Spec
	*Spec
}

func NewService() *Service {
	return &Service{
		Base: services.NewServiceBase(conf.Of(configurations.PluginService)),
		Spec: &Spec{},
	}
}

func main() {
	plugins.Register(
		services.NewFactoryPlugin(conf.Of(configurations.PluginFactoryService), NewFactory()),
		services.NewRuntimePlugin(conf.Of(configurations.PluginRuntimeService), NewRuntime()))
}

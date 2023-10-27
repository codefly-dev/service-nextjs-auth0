package main

import (
	"github.com/codefly-dev/cli/pkg/plugins"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	"github.com/codefly-dev/core/configurations"
)

var conf = configurations.Plugin{
	Publisher:  "codefly.ai",
	Identifier: "go-grpc",
	Kind:       configurations.PluginService,
	Version:    "0.0.0",
}

type Service struct {
	PluginLogger *plugins.PluginLogger
	Location     string
	Spec         *Spec
	GrpcEndpoint configurations.Endpoint
	RestEndpoint *configurations.Endpoint
}

func NewService() *Service {
	return &Service{
		PluginLogger: plugins.NewPluginLogger(conf.Name()),
		Spec:         &Spec{},
	}
}

type Spec struct {
	Debug              bool `yaml:"debug"` // Developer only
	Watch              bool `yaml:"watch"`
	WithDebugSymbols   bool `yaml:"with-debug-symbols"`
	CreateHttpEndpoint bool `yaml:"create-rest-endpoint"`
}

func main() {
	plugins.Register(
		services.NewFactoryPlugin(conf.Of(configurations.PluginFactoryService), NewFactory()),
		services.NewRuntimePlugin(conf.Of(configurations.PluginRuntimeService), NewRuntime()))
}

package main

import (
	"github.com/hygge-io/hygge/pkg/configurations"
	"github.com/hygge-io/hygge/pkg/plugins"
	"github.com/hygge-io/hygge/pkg/plugins/services"
	"path"
)

var conf = configurations.Plugin{
	Base:    "hygge-io/adapters",
	Kind:    configurations.PluginService,
	Version: "0.0.0",
}

type Service struct {
	PluginLogger *plugins.PluginLogger
	Location     string
	Spec         *Spec
	GrpcEndpoint configurations.Endpoint
	RestEndpoint *configurations.Endpoint
}

func (p *Service) Local(f string) string {
	return path.Join(p.Location, f)
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

func (p *Service) InitEndpoints() {
	p.GrpcEndpoint = configurations.Endpoint{
		Name:        configurations.Grpc,
		Description: "Expose gRPC",
	}

	p.PluginLogger.DebugMe("initEndpoints: %v", p.Spec.CreateHttpEndpoint)
	if p.Spec.CreateHttpEndpoint {
		p.RestEndpoint = &configurations.Endpoint{
			Name:        configurations.Rest,
			Description: "Expose REST",
		}
	}
}

func main() {
	plugins.Register(
		services.NewFactoryPlugin(conf.Of(configurations.PluginFactoryService), NewFactory()),
		services.NewRuntimePlugin(conf.Of(configurations.PluginRuntimeService), NewRuntime()))
}

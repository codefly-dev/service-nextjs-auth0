package main

import (
	"embed"
	"github.com/codefly-dev/cli/pkg/plugins/endpoints"
	corev1 "github.com/codefly-dev/cli/proto/v1/core"

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

	// Endpoints
	GrpcEndpoint *corev1.Endpoint
	RestEndpoint *corev1.Endpoint

	// Settings
	*Settings
}

func NewService() *Service {
	return &Service{
		Base:     services.NewServiceBase(plugin.Of(configurations.PluginService)),
		Settings: &Settings{},
	}
}

func (p *Service) LoadEndpoints() error {
	var err error
	for _, ep := range p.Configuration.Endpoints {
		switch ep.Api {
		case configurations.Grpc:
			p.GrpcEndpoint, err = endpoints.NewGrpcApi(ep, p.Local("api.proto"))
			if err != nil {
				return p.Wrapf(err, "cannot create grpc api")
			}
			p.Endpoints = append(p.Endpoints, p.GrpcEndpoint)
			continue
		case configurations.Rest:
			p.RestEndpoint, err = endpoints.NewRestApiFromOpenAPI(p.Context(), ep, p.Local("api.swagger.json"))
			if err != nil {
				return p.Wrapf(err, "cannot create openapi api")
			}
			p.Endpoints = append(p.Endpoints, p.RestEndpoint)
			continue
		}
	}
	return nil
}

func main() {
	plugins.Register(
		services.NewFactoryPlugin(plugin.Of(configurations.PluginFactoryService), NewFactory()),
		services.NewRuntimePlugin(plugin.Of(configurations.PluginRuntimeService), NewRuntime()))
}

//go:embed plugin.codefly.yaml
var info embed.FS

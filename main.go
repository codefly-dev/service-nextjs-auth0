package main

import (
	"context"
	"embed"
	"os"
	"strings"

	basev1 "github.com/codefly-dev/core/generated/go/base/v1"

	"github.com/codefly-dev/core/configurations/standards"

	"github.com/codefly-dev/core/templates"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/codefly-dev/core/agents"
	"github.com/codefly-dev/core/agents/services"
	"github.com/codefly-dev/core/configurations"
	agentv1 "github.com/codefly-dev/core/generated/go/services/agent/v1"
	"github.com/codefly-dev/core/shared"
)

// Agent version
var agent = shared.Must(configurations.LoadFromFs[configurations.Agent](shared.Embed(info)))

type Settings struct {
	DeveloperDebug bool `yaml:"debug"` // Developer only
}

type Service struct {
	*services.Base

	// Settings
	*Settings
	Endpoint *basev1.Endpoint
}

func (s *Service) GetAgentInformation(ctx context.Context, _ *agentv1.AgentInformationRequest) (*agentv1.AgentInformation, error) {
	defer s.Wool.Catch()

	readme, err := templates.ApplyTemplateFrom(shared.Embed(readme), "templates/agent/README.md", s.Information)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &agentv1.AgentInformation{
		Capabilities: []*agentv1.Capability{
			{Type: agentv1.Capability_FACTORY},
			{Type: agentv1.Capability_RUNTIME},
		},
		Languages: []*agentv1.Language{
			{Type: agentv1.Language_TYPESCRIPT},
			{Type: agentv1.Language_JAVASCRIPT},
		},
		Protocols: []*agentv1.Protocol{
			{Type: agentv1.Protocol_HTTP},
		},
		ReadMe: readme,
	}, nil
}

func NewService() *Service {
	return &Service{
		Base:     services.NewServiceBase(context.Background(), agent.Of(configurations.ServiceAgent)),
		Settings: &Settings{},
	}
}

func main() {
	agents.Register(
		services.NewServiceAgent(agent.Of(configurations.ServiceAgent), NewService()),
		services.NewFactoryAgent(agent.Of(configurations.RuntimeServiceAgent), NewFactory()),
		services.NewRuntimeAgent(agent.Of(configurations.FactoryServiceAgent), NewRuntime()))
}

func (s *Service) GetEnv() ([]string, error) {
	// read the env file for auth0
	f, err := os.ReadFile(s.Local("auth0.env"))
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot read auth0.env")
	}
	envs := strings.Split(string(f), "\n")
	return envs, nil
}

func (s *Service) LoadEndpoints(ctx context.Context) error {
	defer s.Wool.Catch()
	var err error
	for _, endpoint := range s.Configuration.Endpoints {
		switch endpoint.API {
		case standards.HTTP:
			s.Endpoint, err = configurations.NewHTTPApi(ctx, endpoint)
			if err != nil {
				return s.Wool.Wrapf(err, "cannot create openapi api")
			}
			s.Endpoints = append(s.Endpoints, s.Endpoint)
			continue
		}
	}
	return nil
}

//go:embed agent.codefly.yaml
var info embed.FS

//go:embed templates/agent
var readme embed.FS

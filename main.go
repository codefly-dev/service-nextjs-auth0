package main

import (
	"context"
	"embed"
	"os"
	"strings"

	"github.com/codefly-dev/core/agents"
	"github.com/codefly-dev/core/shared"

	"github.com/codefly-dev/core/agents/services"
	"github.com/codefly-dev/core/configurations"
	agentv1 "github.com/codefly-dev/core/generated/v1/go/proto/services/agent"
)

// Agent version
var agent = shared.Must(configurations.LoadFromFs[configurations.Agent](shared.Embed(info)))

type Settings struct {
	Debug bool `yaml:"debug"` // Developer only
}

type Service struct {
	*services.Base

	// Settings
	*Settings
}

func (s *Service) GetAgentInformation(ctx context.Context, _ *agentv1.AgentInformationRequest) (*agentv1.AgentInformation, error) {
	return &agentv1.AgentInformation{
		Capabilities: []*agentv1.Capability{
			{Type: agentv1.Capability_FACTORY},
			{Type: agentv1.Capability_RUNTIME},
		},
	}, nil
}

func NewService() *Service {
	return &Service{
		Base:     services.NewServiceBase(shared.NewContext(), agent.Of(configurations.ServiceAgent)),
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
		return nil, s.Wrapf(err, "cannot read auth0.env")
	}
	envs := strings.Split(string(f), "\n")
	return envs, nil
}

//go:embed agent.codefly.yaml
var info embed.FS

package main

import (
	"embed"
	"os"
	"strings"

	"github.com/codefly-dev/core/agents"
	"github.com/codefly-dev/core/shared"

	"github.com/codefly-dev/core/agents/services"
	"github.com/codefly-dev/core/configurations"
)

// Agent version
var agent = configurations.LoadAgentConfiguration(shared.Embed(info))

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
		Base:     services.NewServiceBase(agent.Of(configurations.AgentService)),
		Settings: &Settings{},
	}
}

func main() {
	agents.Register(
		services.NewFactoryAgent(agent.Of(configurations.AgentFactoryService), NewFactory()),
		services.NewRuntimeAgent(agent.Of(configurations.AgentRuntimeService), NewRuntime()))
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

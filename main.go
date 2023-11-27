package main

import (
	"embed"
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

//go:embed agent.codefly.yaml
var info embed.FS

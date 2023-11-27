package main

import (
	"embed"
	"github.com/codefly-dev/core/agents/communicate"
	"github.com/codefly-dev/core/agents/services"
	"github.com/codefly-dev/core/configurations"
	agentsv1 "github.com/codefly-dev/core/proto/v1/go/agents"
	servicev1 "github.com/codefly-dev/core/proto/v1/go/services"
	factoryv1 "github.com/codefly-dev/core/proto/v1/go/services/factory"
	"github.com/codefly-dev/core/runners"
	"github.com/codefly-dev/core/shared"
	"github.com/codefly-dev/core/templates"
)

type Factory struct {
	*Service

	create         *communicate.ClientContext
	createSequence *communicate.Sequence
	Runner         *runners.Runner
}

func NewFactory() *Factory {
	return &Factory{
		Service: NewService(),
	}
}

func (p *Factory) Init(req *servicev1.InitRequest) (*factoryv1.InitResponse, error) {
	defer p.AgentLogger.Catch()

	err := p.Base.Init(req, p.Settings)
	if err != nil {
		return nil, err
	}

	channels, err := p.WithCommunications(services.NewDynamicChannel(communicate.Create))
	if err != nil {
		return nil, err
	}

	readme, err := templates.ApplyTemplateFrom(shared.Embed(factory), "templates/factory/README.md", p.Information)
	if err != nil {
		return nil, err
	}

	return &factoryv1.InitResponse{
		Version:   p.Version(),
		Endpoints: p.Endpoints,
		Channels:  channels,
		ReadMe:    readme,
	}, nil
}

const Watch = "watch"
const WithRest = "with_rest"

func (p *Factory) NewCreateCommunicate() (*communicate.ClientContext, error) {
	client, err := communicate.NewClientContext(p.Context(), communicate.Create)
	p.createSequence, err = client.NewSequence(
		client.NewConfirm(&agentsv1.Message{Name: Watch, Message: "Code hot-reload (Recommended)?", Description: "codefly can restart your service when code changes are detected 🔎"}, true),
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}

type Deployment struct {
	Replicas int
}

type CreateConfiguration struct {
	Image      *configurations.DockerImage
	Deployment Deployment
	Domain     string
	Envs       []string
}

func (p *Factory) Create(req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer p.AgentLogger.Catch()

	if p.create == nil {
		// Initial setup
		var err error
		p.AgentLogger.DebugMe("Setup communication")
		p.create, err = p.NewCreateCommunicate()
		if err != nil {
			return nil, p.AgentLogger.Wrapf(err, "cannot setup up communication")
		}
		err = p.Wire(communicate.Create, p.create)
		if err != nil {
			return nil, p.AgentLogger.Wrapf(err, "cannot wire communication")
		}
		return &factoryv1.CreateResponse{NeedCommunication: true}, nil
	}

	// Make sure the communication for create has been done successfully
	if !p.create.Ready() {
		p.DebugMe("create not ready!")
		return nil, p.AgentLogger.Errorf("create: communication not ready")
	}

	p.Settings.Watch = p.create.Confirm(p.createSequence.Find(Watch)).Confirmed

	ignores := []string{"node_modules", ".next", ".idea"}

	err := p.Templates(p.Information, services.WithFactory(factory, ignores...), services.WithBuilder(builder))
	if err != nil {
		return nil, p.Wrapf(err, "cannot copy and apply template")
	}
	// Need to handle the case of pages/_app.tsx
	err = templates.Copy(shared.Embed(special),
		shared.NewFile("templates/special/pages/app.tsx"), shared.NewFile(p.Local("pages/_app.tsx")))
	if err != nil {
		return nil, p.Wrapf(err, "cannot copy special template")
	}

	out, err := shared.GenerateTree(p.Location, " ")
	if err != nil {
		return nil, err
	}
	p.AgentLogger.Info("tree: %s", out)

	p.Runner = &runners.Runner{
		Name:          p.Service.Identity.Name,
		Bin:           "npm",
		Args:          []string{"install", "ci"},
		AgentLogger:   p.AgentLogger,
		ServiceLogger: p.ServiceLogger,
		Dir:           p.Location,
		Debug:         p.Debug,
	}
	err = p.Runner.Init(p.Context())
	if err != nil {
		return nil, p.Wrapf(err, "cannot start service")
	}
	p.DebugMe("running npm install")
	_, err = p.Runner.Run(p.Context())
	if err != nil {
		return nil, p.Wrapf(err, "cannot start go program")
	}

	return p.Base.Create(p.Settings, p.Endpoints...)
}

func (p *Factory) Update(req *factoryv1.UpdateRequest) (*factoryv1.UpdateResponse, error) {
	defer p.AgentLogger.Catch()

	return &factoryv1.UpdateResponse{}, nil
}

func (p *Factory) Sync(req *factoryv1.SyncRequest) (*factoryv1.SyncResponse, error) {
	defer p.AgentLogger.Catch()

	return &factoryv1.SyncResponse{}, nil
}

type Env struct {
	Key   string
	Value string
}

type DockerTemplating struct {
	Envs []Env
}

func (p *Factory) Build(req *factoryv1.BuildRequest) (*factoryv1.BuildResponse, error) {
	p.AgentLogger.Debugf("building docker image")

	return &factoryv1.BuildResponse{}, nil
}

func (p *Factory) Deploy(req *factoryv1.DeploymentRequest) (*factoryv1.DeploymentResponse, error) {
	return &factoryv1.DeploymentResponse{}, nil
}

func (p *Factory) CreateEndpoints() error {

	return nil
}

//go:embed templates/routes
var routes embed.FS

//go:embed templates/factory
var factory embed.FS

//go:embed templates/builder
var builder embed.FS

//go:embed templates/special
var special embed.FS

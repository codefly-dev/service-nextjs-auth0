package main

import (
	"embed"
	"strings"

	dockerhelpers "github.com/codefly-dev/core/agents/helpers/docker"

	"os"

	"github.com/codefly-dev/core/agents/communicate"
	"github.com/codefly-dev/core/agents/endpoints"
	"github.com/codefly-dev/core/agents/network"
	"github.com/codefly-dev/core/agents/services"
	"github.com/codefly-dev/core/configurations"
	agentsv1 "github.com/codefly-dev/core/proto/v1/go/agents"
	basev1 "github.com/codefly-dev/core/proto/v1/go/base"
	servicev1 "github.com/codefly-dev/core/proto/v1/go/services"
	factoryv1 "github.com/codefly-dev/core/proto/v1/go/services/factory"
	runtimev1 "github.com/codefly-dev/core/proto/v1/go/services/runtime"
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

type CreateConfiguration struct {
	Image  *configurations.DockerImage
	Domain string
	Envs   []string
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
	p.DebugMe("got dependency group %v", endpoints.CondensedOutput(req.DependencyEndpointGroup))

	err := os.Remove(p.Local("codefly/builder/Dockerfile"))
	if err != nil {
		return nil, p.Wrapf(err, "cannot remove dockerfile")
	}
	err = p.Templates(nil, services.WithBuilder(builder))
	if err != nil {
		return nil, p.Wrapf(err, "cannot copy and apply template")
	}
	builder, err := dockerhelpers.NewBuilder(dockerhelpers.BuilderConfiguration{
		Root:       p.Location,
		Dockerfile: "codefly/builder/Dockerfile",
		Image:      p.DockerImage().Name,
		Tag:        p.DockerImage().Tag,
	})
	if err != nil {
		return nil, p.Wrapf(err, "cannot create builder")
	}
	builder.WithLogger(p.AgentLogger)
	_, err = builder.Build()
	if err != nil {
		return nil, p.Wrapf(err, "cannot build image")
	}
	return &factoryv1.BuildResponse{}, nil
}

type Deployment struct {
	Replicas int
}

type DeploymentParameter struct {
	Image *configurations.DockerImage
	*services.Information
	Deployment
	ConfigMap map[string]string
}

func EnvsAsMap(envs []string) map[string]string {
	m := make(map[string]string)
	for _, env := range envs {
		split := strings.SplitN(env, "=", 2)
		if len(split) == 2 {
			m[split[0]] = split[1]
		}
	}
	return m
}

func (p *Factory) Deploy(req *factoryv1.DeploymentRequest) (*factoryv1.DeploymentResponse, error) {
	defer p.AgentLogger.Catch()

	// We want to use DNS to create NetworkMapping
	networkMapping, err := p.Network(endpoints.FlattenEndpoints(p.Context(), req.DependencyEndpointGroup))
	if err != nil {
		return nil, p.Wrapf(err, "cannot create network mapping")
	}

	nws, err := network.ConvertToEnvironmentVariables(networkMapping)
	if err != nil {
		return nil, p.Wrapf(err, "cannot convert network mappings")
	}
	local := EnvLocal{Envs: nws}
	// Append Auth0
	auth0, err := p.GetEnv()
	if err != nil {
		return nil, p.Wrapf(err, "cannot get env")
	}
	local.Envs = append(local.Envs, auth0...)

	deploy := DeploymentParameter{ConfigMap: EnvsAsMap(local.Envs), Image: p.DockerImage(), Information: p.Information, Deployment: Deployment{Replicas: 1}}
	err = p.Templates(deploy,
		services.WithDeploymentFor(deployment, "kustomize/base", templates.WithOverrideAll()),
		services.WithDeploymentFor(deployment, "kustomize/overlays/environment",
			services.WithDestination("kustomize/overlays/%s", req.Environment.Name), templates.WithOverrideAll()),
	)
	if err != nil {
		return nil, err
	}
	return &factoryv1.DeploymentResponse{}, nil
}

func (p *Factory) Network(es []*basev1.Endpoint) ([]*runtimev1.NetworkMapping, error) {
	p.DebugMe("in network: %v", endpoints.Condensed(es))
	pm, err := network.NewServiceDnsManager(p.Context(), p.Identity)
	if err != nil {
		return nil, p.Wrapf(err, "cannot create network manager")
	}
	for _, endpoint := range es {
		err = pm.Expose(endpoint)
		if err != nil {
			return nil, p.Wrapf(err, "cannot add grpc endpoint to network manager")
		}
	}
	err = pm.Reserve()
	if err != nil {
		return nil, p.Wrapf(err, "cannot reserve ports")
	}
	return pm.NetworkMapping()
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

//go:embed templates/deployment
var deployment embed.FS

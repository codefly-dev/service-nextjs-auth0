package main

import (
	"embed"
	"fmt"
	"github.com/codefly-dev/cli/pkg/plugins/communicate"
	"github.com/codefly-dev/cli/pkg/plugins/endpoints"
	corev1 "github.com/codefly-dev/cli/proto/v1/core"
	"os"

	dockerhelpers "github.com/codefly-dev/cli/pkg/plugins/helpers/docker"
	golanghelpers "github.com/codefly-dev/cli/pkg/plugins/helpers/go"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	v1 "github.com/codefly-dev/cli/proto/v1/services"
	factoryv1 "github.com/codefly-dev/cli/proto/v1/services/factory"
	"github.com/codefly-dev/core/configurations"
	"github.com/codefly-dev/core/shared"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Factory struct {
	*Service

	create         *communicate.ClientContext
	createSequence *communicate.Sequence
}

func NewFactory() *Factory {
	return &Factory{
		Service: NewService(),
	}
}

type Proto struct {
	Package      string
	PackageAlias string
}

type CreateService struct {
	Name      string
	TitleName string
	Proto     Proto
	Go        GenerateInstructions
}

type GenerateInstructions struct {
	Package string
}

type Readme struct {
	Summary string
}

type CreateConfiguration struct {
	Name        string
	Destination string
	Namespace   string
	Domain      string
	Service     CreateService
	Plugin      configurations.Plugin
	Readme      Readme
}

func (p *Factory) Init(req *v1.InitRequest) (*factoryv1.InitResponse, error) {
	defer p.PluginLogger.Catch()

	err := p.Base.Init(req, p.Settings)
	if err != nil {
		return nil, err
	}

	err = p.LoadEndpoints()
	if err != nil {
		return nil, err
	}

	p.create, err = p.NewCreateCommunicate()
	if err != nil {
		return nil, err
	}

	channels, err := p.WithCommunications(services.NewChannel(communicate.Create, p.create))
	if err != nil {
		return nil, err
	}
	return &factoryv1.InitResponse{
		Version:  p.Version(),
		Channels: channels,
	}, nil
}

const Watch = "watch"
const WithRest = "with_rest"

func (p *Factory) Welcome() (*corev1.Message, map[string]string) {
	return &corev1.Message{Message: `Welcome to the service plugin #(bold,cyan)[go-grc] by plugin #(bold,cyan)[codefly.ai]
Some of the things this plugin provides for you:
- gRPC server
- REST server auto-generated (optional)
- hot-reload (optional)
- docker build
- Kubernetes deployment
Are you excited?`}, map[string]string{
			"PluginName":      plugin.Identifier,
			"PluginPublisher": plugin.Publisher,
		}
}

func (p *Factory) NewCreateCommunicate() (*communicate.ClientContext, error) {
	client, err := communicate.NewClientContext(p.Context(), communicate.Create)
	p.createSequence, err = client.NewSequence(
		client.Display(p.Welcome()),
		client.NewConfirm(&corev1.Message{Name: Watch, Message: "Code hot-reload (Recommended)?", Description: "codefly can restart your service when code changes are detected"}, true),
		client.NewConfirm(&corev1.Message{Name: WithRest, Message: "Automatic REST generation (Recommended)?", Description: "codefly can generate a REST server that stays magically synced to your gRPC definition -- the easiest way to do REST"}, true),
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (p *Factory) Create(req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer p.PluginLogger.Catch()

	// Make sure the communication for create has been done successfully
	if !p.create.Ready() {
		return nil, p.PluginLogger.Errorf("create: communication not ready")
	}

	p.Settings.Watch = p.create.Confirm(p.createSequence.Find(Watch)).Confirmed
	p.Settings.CreateHttpEndpoint = p.create.Confirm(p.createSequence.Find(WithRest)).Confirmed

	create := CreateConfiguration{
		Name:      cases.Title(language.English, cases.NoLower).String(p.Identity.Name),
		Domain:    p.Identity.Domain,
		Namespace: p.Identity.Namespace,
		Readme:    Readme{Summary: p.Identity.Name},
	}

	ignores := []string{"go.work", "service.generation.codefly.yaml"}
	err := p.Templates(create, services.WithFactory(factory, ignores...))
	if err != nil {
		return nil, err
	}

	out, err := shared.GenerateTree(p.Location, " ")
	if err != nil {
		return nil, err
	}
	p.PluginLogger.Info("tree: %s", out)

	err = p.CreateEndpoints()
	if err != nil {
		return nil, p.Wrapf(err, "cannot create endpoints")
	}

	helper := golanghelpers.Go{Dir: p.Location}

	err = helper.BufGenerate(p.PluginLogger)
	if err != nil {
		return nil, fmt.Errorf("factory>create: go helper: cannot run buf generate: %v", err)
	}
	err = helper.ModTidy(p.PluginLogger)
	if err != nil {
		return nil, fmt.Errorf("factory>create: go helper: cannot run mod tidy: %v", err)
	}

	return p.Base.Create(p.Settings, p.Endpoints...)
}

func (p *Factory) Update(req *factoryv1.UpdateRequest) (*factoryv1.UpdateResponse, error) {
	defer p.PluginLogger.Catch()

	p.ServiceLogger.Info("Updating")

	err := p.Base.Templates(nil, services.WithBuilder(builder))
	if err != nil {
		return nil, p.Wrapf(err, "cannot copy and apply template")
	}

	helper := golanghelpers.Go{Dir: p.Location}
	err = helper.Update(p.PluginLogger)
	if err != nil {
		return nil, fmt.Errorf("factory>update: go helper: cannot run update: %v", err)
	}
	return &factoryv1.UpdateResponse{}, nil
}

func (p *Factory) Sync(req *factoryv1.SyncRequest) (*factoryv1.SyncResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.TODO("Some caching please!")

	p.PluginLogger.Debugf("running sync: %v", p.Location)
	helper := golanghelpers.Go{Dir: p.Location}

	// Clean-up the generated code
	p.PluginLogger.TODO("get location of generated code from buf")
	err := os.RemoveAll(p.Local("adapters/v1"))
	if err != nil {
		return nil, p.Wrapf(err, "cannot remove adapters")
	}
	// Re-generate
	err = helper.BufGenerate(p.PluginLogger)
	if err != nil {
		return nil, p.Wrapf(err, "cannot generate proto")
	}
	err = helper.ModTidy(p.PluginLogger)
	if err != nil {
		return nil, p.Wrapf(err, "cannot tidy go.mod")
	}

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
	p.PluginLogger.Debugf("building docker image")
	docker := DockerTemplating{}

	e, err := endpoints.FromProtoEndpoint(p.GrpcEndpoint)
	if err != nil {
		return nil, p.Wrapf(err, "cannot convert grpc endpoint")
	}
	gRPC := configurations.AsEndpointEnvironmentVariableKey(p.Configuration.Application, p.Configuration.Name, e)
	docker.Envs = append(docker.Envs, Env{Key: gRPC, Value: "localhost:9090"})
	if p.RestEndpoint != nil {
		e, err = endpoints.FromProtoEndpoint(p.RestEndpoint)
		if err != nil {
			return nil, p.Wrapf(err, "cannot convert grpc endpoint")
		}
		rest := configurations.AsEndpointEnvironmentVariableKey(p.Configuration.Application, p.Configuration.Name, e)
		docker.Envs = append(docker.Envs, Env{Key: rest, Value: "localhost:8080"})
	}

	err = os.Remove(p.Local("codefly/builder/Dockerfile"))
	if err != nil {
		return nil, p.Wrapf(err, "cannot remove dockerfile")
	}
	err = p.Templates(docker, services.WithBuilder(builder))
	if err != nil {
		return nil, p.Wrapf(err, "cannot copy and apply template")
	}
	builder, err := dockerhelpers.NewBuilder(dockerhelpers.BuilderConfiguration{
		Root:       p.Location,
		Dockerfile: "codefly/builder/Dockerfile",
		Image:      p.Identity.Name,
		Tag:        p.Configuration.Version,
	})
	if err != nil {
		return nil, p.Wrapf(err, "cannot create builder")
	}
	builder.WithLogger(p.PluginLogger)
	_, err = builder.Build()
	if err != nil {
		return nil, p.Wrapf(err, "cannot build image")
	}
	return &factoryv1.BuildResponse{}, nil
}

func (p *Factory) Deploy(req *factoryv1.DeploymentRequest) (*factoryv1.DeploymentResponse, error) {
	return &factoryv1.DeploymentResponse{}, nil
}

func (p *Factory) CreateEndpoints() error {
	grpc, err := endpoints.NewGrpcApi(&configurations.Endpoint{Name: "grpc"}, p.Local("api.proto"))
	if err != nil {
		return p.Wrapf(err, "cannot create grpc api")
	}
	p.Endpoints = append(p.Endpoints, grpc)

	if p.Settings.CreateHttpEndpoint {
		rest, err := endpoints.NewRestApiFromOpenAPI(p.Context(), &configurations.Endpoint{Name: "rest", Public: true}, p.Local("api.swagger.json"))
		if err != nil {
			return p.Wrapf(err, "cannot create openapi api")
		}
		p.Endpoints = append(p.Endpoints, rest)
	}
	return nil
}

//go:embed templates/factory
var factory embed.FS

//go:embed templates/builder
var builder embed.FS

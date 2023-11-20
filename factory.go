package main

import (
	"bytes"
	"embed"
	"fmt"
	"github.com/codefly-dev/cli/pkg/plugins/communicate"
	"github.com/codefly-dev/cli/pkg/plugins/endpoints"
	corev1 "github.com/codefly-dev/cli/proto/v1/core"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"
	"strings"
	"unicode"

	dockerhelpers "github.com/codefly-dev/cli/pkg/plugins/helpers/docker"
	golanghelpers "github.com/codefly-dev/cli/pkg/plugins/helpers/go"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	v1 "github.com/codefly-dev/cli/proto/v1/services"
	factoryv1 "github.com/codefly-dev/cli/proto/v1/services/factory"
	"github.com/codefly-dev/core/configurations"
	"github.com/codefly-dev/core/shared"
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

	channels, err := p.WithCommunications(services.NewDynamicChannel(communicate.Create))
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
const WithKreya = "with_kreya"
const WithPostman = "with_postman"

func (p *Factory) Welcome() (*corev1.Message, map[string]string) {
	return &corev1.Message{Message: `Welcome to the service plugin #(bold,cyan)[go-grc] by plugin #(bold,cyan)[codefly.ai]
Some of the things this plugin provides for you:
 #(bold,cyan)[Developer Experience]
- hot-reload
- code generation automated
- Kreya configuration (optional)
- Postman configuration (coming soon)
 #(bold,cyan)[Code]
- gRPC server
- REST server auto-generated (optional)
- Version endpoint
#(bold,cyan)[Production ready]
- docker build
- Kubernetes deployment
`}, map[string]string{
			"PluginName":      plugin.Identifier,
			"PluginPublisher": plugin.Publisher,
		}
}

func (p *Factory) NewCreateCommunicate() (*communicate.ClientContext, error) {
	client, err := communicate.NewClientContext(p.Context(), communicate.Create)
	p.createSequence, err = client.NewSequence(
		client.Display(p.Welcome()),
		client.NewConfirm(&corev1.Message{Name: Watch, Message: "Code hot-reload (Recommended)?", Description: "codefly can restart your service when code changes are detected 🔎"}, true),
		client.NewConfirm(&corev1.Message{Name: WithRest, Message: "Automatic REST generation (Recommended)?", Description: "codefly can generate a REST server that stays magically 🪄 synced to your gRPC definition -- the easiest way to do REST"}, true),
		client.NewConfirm(&corev1.Message{Name: WithRest, Message: "Kreya configuration?", Description: "codefly can create a Kreya configuration to make it easy to call your endpoints, because why would you want to do that manually? 😵‍💫"}, true),
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}

type Deployment struct {
	Replicas int
}

type DockerImage struct {
	Repository string
	Name       string
	Tag        string
}

type Case struct {
	LowerCase string
	SnakeCase string
	CamelCase string
	KebabCase string
	Title     string
	DnsCase   string
}

type ServiceWithCase struct {
	Name      Case
	Unique    Case
	Domain    string
	Namespace string
}

// toSnakeCase converts a string to snake_case
func toSnakeCase(s string) string {
	var buf bytes.Buffer
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				buf.WriteRune('_')
			}
			buf.WriteRune(unicode.ToLower(r))
		} else {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// toCamelCase converts a string to camelCase
func toCamelCase(s string) string {
	var buf bytes.Buffer
	toUpper := false
	for _, r := range s {
		if r == '_' || r == '-' {
			toUpper = true
			continue
		}
		if toUpper {
			buf.WriteRune(unicode.ToUpper(r))
			toUpper = false
		} else {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// toKebabCase converts a string to kebab-case
func toKebabCase(str string) string {
	return strings.ReplaceAll(toSnakeCase(str), "_", "-")
}

func toDnsCase(s string) string {
	return strings.ReplaceAll(toLowerCase(s), "/", "-")
}

func toCase(s string) Case {
	return Case{
		LowerCase: toLowerCase(s),
		DnsCase:   toDnsCase(s),
		SnakeCase: toSnakeCase(s),
		CamelCase: toCamelCase(s),
		KebabCase: toKebabCase(s),
		Title:     cases.Title(language.English).String(s),
	}
}

func toLowerCase(s string) string {
	return strings.ToLower(s)
}

func ToServiceWithCase(svc *configurations.Service) ServiceWithCase {
	return ServiceWithCase{
		Name:      toCase(svc.Name),
		Unique:    toCase(svc.Unique()),
		Domain:    svc.Domain,
		Namespace: svc.Namespace,
	}
}

type Readme struct {
	Summary string
}

type CreateConfiguration struct {
	Service    ServiceWithCase
	Plugin     *configurations.Plugin
	Image      DockerImage
	Readme     Readme
	Deployment Deployment
}

func (p *Factory) Create(req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer p.PluginLogger.Catch()

	if p.create == nil {
		// Initial setup
		var err error
		p.PluginLogger.DebugMe("Setup communication")
		p.create, err = p.NewCreateCommunicate()
		if err != nil {
			return nil, p.PluginLogger.Wrapf(err, "cannot setup up communication")
		}
		err = p.Wire(communicate.Create, p.create)
		if err != nil {
			return nil, p.PluginLogger.Wrapf(err, "cannot wire communication")
		}
		return &factoryv1.CreateResponse{NeedCommunication: true}, nil
	}

	// Make sure the communication for create has been done successfully
	if !p.create.Ready() {
		p.DebugMe("create not ready!")
		return nil, p.PluginLogger.Errorf("create: communication not ready")
	}

	p.Settings.Watch = p.create.Confirm(p.createSequence.Find(Watch)).Confirmed
	p.Settings.CreateHttpEndpoint = p.create.Confirm(p.createSequence.Find(WithRest)).Confirmed

	create := CreateConfiguration{
		Service:    ToServiceWithCase(p.Configuration),
		Plugin:     p.Plugin,
		Image:      DockerImage{},
		Deployment: Deployment{},
		Readme:     Readme{Summary: p.Identity.Name},
	}

	ignores := []string{"go.work", "service.generation.codefly.yaml"}
	err := p.Templates(create, services.WithFactory(factory, ignores...),
		services.WithBuilder(builder),
		services.WithDeploymentFor(deployment, "kustomize/base"))
	if err != nil {
		return nil, err
	}

	out, err := shared.GenerateTree(p.Location, " ")
	if err != nil {
		return nil, err
	}
	p.PluginLogger.Info("tree: %s", out)
	p.ServiceLogger.Info("We generated this code for you:\n%s", out)

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

//go:embed templates/deployment
var deployment embed.FS

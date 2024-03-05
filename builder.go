package main

import (
	"context"
	"embed"
	dockerhelpers "github.com/codefly-dev/core/agents/helpers/docker"
	"github.com/codefly-dev/core/agents/services"
	"github.com/codefly-dev/core/configurations"
	basev0 "github.com/codefly-dev/core/generated/go/base/v0"
	builderv0 "github.com/codefly-dev/core/generated/go/services/builder/v0"
	"github.com/codefly-dev/core/wool"
	"os"

	"github.com/codefly-dev/core/runners"
	"github.com/codefly-dev/core/shared"
	"github.com/codefly-dev/core/templates"
)

type Builder struct {
	*Service

	EnvironmentVariables *configurations.EnvironmentVariableManager
	NetworkMappings      []*basev0.NetworkMapping
}

func NewBuilder() *Builder {
	return &Builder{
		Service: NewService(),
	}
}

func (s *Builder) Load(ctx context.Context, req *builderv0.LoadRequest) (*builderv0.LoadResponse, error) {
	defer s.Wool.Catch()

	err := s.Builder.Load(ctx, req.Identity, s.Settings)
	if err != nil {
		return nil, err
	}

	gettingStarted, err := templates.ApplyTemplateFrom(ctx, shared.Embed(factoryFS), "templates/factory/GETTING_STARTED.md", s.Information)
	if err != nil {
		return s.Builder.LoadError(err)
	}

	s.EnvironmentVariables = configurations.NewEnvironmentVariableManager()

	return s.Builder.LoadResponse(gettingStarted)
}

func (s *Builder) Init(ctx context.Context, req *builderv0.InitRequest) (*builderv0.InitResponse, error) {
	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	s.NetworkMappings = req.ProposedNetworkMappings

	s.Wool.In("Init").Debug("dependencies", wool.SliceCountField(req.DependenciesEndpoints))

	auth0, err := configurations.FindProjectProvider(s.Settings.Auth0Provider, req.ProviderInfos)
	if err != nil {
		return s.Builder.InitError(err)
	}

	env := configurations.ProviderInformationAsEnvironmentVariables(auth0)
	s.EnvironmentVariables.Add(env...)

	s.DependencyEndpoints = req.DependenciesEndpoints
	hash, err := requirements.Hash(ctx)
	if err != nil {
		return s.Builder.InitError(err)
	}

	return s.Builder.InitResponse(s.NetworkMappings, hash)
}

func (s *Builder) Update(ctx context.Context, req *builderv0.UpdateRequest) (*builderv0.UpdateResponse, error) {
	defer s.Wool.Catch()

	return &builderv0.UpdateResponse{}, nil
}

func (s *Builder) Sync(ctx context.Context, req *builderv0.SyncRequest) (*builderv0.SyncResponse, error) {
	defer s.Wool.Catch()

	return s.Builder.SyncResponse()
}

type Env struct {
	Key   string
	Value string
}

type DockerTemplating struct {
	Envs       []Env
	Components []string
}

func (s *Builder) Build(ctx context.Context, req *builderv0.BuildRequest) (*builderv0.BuildResponse, error) {
	s.Wool.Debug("building docker image")
	ctx = s.Wool.Inject(ctx)

	image := s.DockerImage(req.BuildContext)

	s.Wool.In("Build").Debug("dependencies", wool.SliceCountField(s.DependencyEndpoints))

	docker := DockerTemplating{
		Components: requirements.All(),
	}

	err := shared.DeleteFile(ctx, s.Local("codefly/builder/Dockerfile"))
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot remove dockerfile")
	}

	err = s.Templates(ctx, docker, services.WithBuilder(builderFS))
	if err != nil {
		return s.Builder.BuildError(err)
	}

	builder, err := dockerhelpers.NewBuilder(dockerhelpers.BuilderConfiguration{
		Root:        s.Location,
		Dockerfile:  "codefly/builder/Dockerfile",
		Destination: image,
		Output:      s.Wool,
	})
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot create builder")
	}
	_, err = builder.Build(ctx)
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot build image")
	}
	return &builderv0.BuildResponse{}, nil
}

func (s *Builder) Deploy(ctx context.Context, req *builderv0.DeploymentRequest) (*builderv0.DeploymentResponse, error) {
	defer s.Wool.Catch()

	publicNetworkMappings := configurations.ExtractPublicNetworkMappings(req.NetworkMappings)

	envs, err := configurations.ExtractEndpointEnvironmentVariables(ctx, publicNetworkMappings)
	if err != nil {
		return s.Builder.DeployError(err)
	}

	restEnvs, err := configurations.ExtractRestRoutesEnvironmentVariables(ctx, publicNetworkMappings)
	if err != nil {
		return s.Builder.DeployError(err)
	}

	envs = append(envs, restEnvs...)

	endpoints := services.EnvsAsConfigMapData(envs)

	auth0 := services.EnvsAsSecretData(s.EnvironmentVariables.GetBase())
	params := services.DeploymentParameter{SecretMap: auth0, ConfigMap: endpoints}

	err = s.Builder.Deploy(ctx, req, deploymentFS, params)
	if err != nil {
		return s.Builder.DeployError(err)
	}
	return s.Builder.DeployResponse()
}

type CreateConfiguration struct {
	Image  *configurations.DockerImage
	Domain string
	Envs   []string
}

func (s *Builder) Create(ctx context.Context, req *builderv0.CreateRequest) (*builderv0.CreateResponse, error) {
	defer s.Wool.Catch()

	err := s.CreateEndpoint(ctx)
	if err != nil {
		return s.Builder.CreateError(err)
	}

	ignore := shared.NewIgnore("node_modules", ".next", ".idea")
	err = s.Templates(ctx, s.Information, services.WithFactory(factoryFS).WithPathSelect(ignore))

	if err != nil {
		return s.Builder.CreateError(err)
	}

	// Need to handle the case of pages/_aps.tsx
	err = templates.Copy(ctx, shared.Embed(specialFS),
		"templates/special/pages/app.tsx",
		s.Local("pages/_app.tsx"))
	if err != nil {
		return s.Builder.CreateError(err)
	}

	s.Wool.Debug("removing node_modules")
	err = os.RemoveAll(s.Local("node_modules"))
	if err != nil {
		return s.Builder.CreateError(err)
	}

	s.Wool.Debug("npm install")

	s.Wool.Info("installing npm dependencies. Coffee time! ☕️")
	runner, err := runners.NewRunner(ctx, "npm", "install", "ci")
	if err != nil {
		return s.Builder.CreateError(err)
	}
	runner.WithDir(s.Location)

	err = runner.Run()
	if err != nil {
		return s.Builder.CreateError(err)
	}

	s.Wool.Debug("npm install done")

	s.Configuration.ProviderDependencies = []string{"auth0"}

	return s.Builder.CreateResponse(ctx, s.Settings)
}

func (s *Builder) CreateEndpoint(ctx context.Context) error {
	http, err := configurations.NewHTTPApi(ctx, &configurations.Endpoint{Name: "web", Visibility: configurations.VisibilityPublic})
	if err != nil {
		return s.Wool.Wrapf(err, "cannot create HTTP api")
	}
	s.Endpoints = append(s.Endpoints, http)
	return nil
}

//go:embed templates/factory
var factoryFS embed.FS

//go:embed templates/builder
var builderFS embed.FS

//go:embed templates/deployment
var deploymentFS embed.FS

//go:embed templates/special
var specialFS embed.FS

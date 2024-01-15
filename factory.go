package main

import (
	"context"
	"embed"
	"os"
	"strings"

	"github.com/codefly-dev/core/agents/network"
	"github.com/codefly-dev/core/agents/services"
	"github.com/codefly-dev/core/builders"
	"github.com/codefly-dev/core/configurations"
	basev0 "github.com/codefly-dev/core/generated/go/base/v0"
	factoryv0 "github.com/codefly-dev/core/generated/go/services/factory/v0"
	runtimev0 "github.com/codefly-dev/core/generated/go/services/runtime/v0"

	dockerhelpers "github.com/codefly-dev/core/agents/helpers/docker"

	"github.com/codefly-dev/core/runners"
	"github.com/codefly-dev/core/shared"
	"github.com/codefly-dev/core/templates"
)

type Factory struct {
	*Service

	Runner               *runners.Runner
	EnvironmentVariables *configurations.EnvironmentVariableManager
}

func NewFactory() *Factory {
	return &Factory{
		Service: NewService(),
	}
}

func (s *Factory) Load(ctx context.Context, req *factoryv0.LoadRequest) (*factoryv0.LoadResponse, error) {
	defer s.Wool.Catch()

	err := s.Factory.Load(ctx, req.Identity, s.Settings)
	if err != nil {
		return nil, err
	}

	gettingStarted, err := templates.ApplyTemplateFrom(shared.Embed(factory), "templates/factory/GETTING_STARTED.md", s.Information)
	if err != nil {
		return nil, err
	}

	s.EnvironmentVariables = configurations.NewEnvironmentVariableManager()

	return &factoryv0.LoadResponse{
		Version:        s.Version(),
		Endpoints:      s.Endpoints,
		GettingStarted: gettingStarted,
	}, nil
}

type CreateConfiguration struct {
	Image  *configurations.DockerImage
	Domain string
	Envs   []string
}

func (s *Factory) Create(ctx context.Context, req *factoryv0.CreateRequest) (*factoryv0.CreateResponse, error) {
	defer s.Wool.Catch()

	err := s.CreateEndpoint(ctx)
	if err != nil {
		return s.Factory.CreateError(err)
	}
	ignores := []string{"node_modules", ".next", ".idea"}

	err = s.Templates(ctx, s.Information, services.WithFactory(factory, ignores...))
	if err != nil {
		return s.Factory.CreateError(err)
	}

	// Need to handle the case of pages/_aps.tsx
	err = templates.Copy(ctx, shared.Embed(special),
		shared.NewFile("templates/special/pages/app.tsx"),
		shared.NewFile(s.Local("pages/_app.tsx")))
	if err != nil {
		return s.Factory.CreateError(err)
	}

	s.Wool.Debug("removing node_modules")
	err = os.RemoveAll(s.Local("node_modules"))
	if err != nil {
		return s.Factory.CreateError(err)
	}

	s.Wool.Debug("npm install")

	s.Runner = &runners.Runner{
		Name: s.Service.Identity.Name,
		Bin:  "npm",
		Args: []string{"install", "ci"},
		Dir:  s.Location,
		Envs: os.Environ(),
	}

	err = s.Runner.Run(ctx)
	if err != nil {
		return s.Factory.CreateError(err)
	}

	s.Wool.Debug("npm install done")

	return s.Factory.CreateResponse(ctx, s.Settings, s.Endpoints...)
}

func (s *Factory) Init(ctx context.Context, req *factoryv0.InitRequest) (*factoryv0.InitResponse, error) {
	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	s.DependencyEndpoints = req.DependenciesEndpoints

	hash, err := requirements.Hash(ctx)
	if err != nil {
		return s.Factory.InitError(err)
	}

	return s.Factory.InitResponse(hash)
}

func (s *Factory) Update(ctx context.Context, req *factoryv0.UpdateRequest) (*factoryv0.UpdateResponse, error) {
	defer s.Wool.Catch()

	return &factoryv0.UpdateResponse{}, nil
}

func (s *Factory) Sync(ctx context.Context, req *factoryv0.SyncRequest) (*factoryv0.SyncResponse, error) {
	defer s.Wool.Catch()

	return &factoryv0.SyncResponse{}, nil
}

type Env struct {
	Key   string
	Value string
}

type DockerTemplating struct {
	Envs       []Env
	Dependency builders.Dependency
}

func (s *Factory) Build(ctx context.Context, req *factoryv0.BuildRequest) (*factoryv0.BuildResponse, error) {
	s.Wool.Debug("building docker image")
	ctx = s.Wool.Inject(ctx)

	docker := DockerTemplating{
		Dependency: *requirements,
	}

	// We want to use DNS to create NetworkMapping
	networkMapping, err := s.Network(s.DependencyEndpoints)
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot create network mapping")
	}

	nws, err := network.ConvertToEnvironmentVariables(networkMapping)
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot convert network mappings")
	}

	s.EnvironmentVariables.Add(nws...)

	//for _, inf
	//if err != nil {
	//	return nil, s.Wool.Wrapf(err, "cannot get env")
	//}
	//local.Envs = append(local.Envs, auth0...)
	//
	// Generate the .env.local
	err = templates.CopyAndApplyTemplate(ctx, shared.Embed(special),
		shared.NewFile("templates/special/env.local.tmpl"), shared.NewFile(s.Local(".env.local")), s.EnvironmentVariables.Get())
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot copy special template")
	}

	err = shared.DeleteFile(ctx, s.Local("codefly/builder/Dockerfile"))
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot remove dockerfile")
	}

	err = s.Templates(ctx, docker, services.WithBuilder(builder))
	if err != nil {
		return s.Factory.BuildError(err)
	}

	builder, err := dockerhelpers.NewBuilder(dockerhelpers.BuilderConfiguration{
		Root:        s.Location,
		Dockerfile:  "codefly/builder/Dockerfile",
		Destination: s.DockerImage(),
	})
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot create builder")
	}
	_, err = builder.Build(ctx)
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot build image")
	}
	return &factoryv0.BuildResponse{}, nil
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

func (s *Factory) Deploy(ctx context.Context, req *factoryv0.DeploymentRequest) (*factoryv0.DeploymentResponse, error) {
	defer s.Wool.Catch()
	//
	//// We want to use DNS to create NetworkMapping
	//networkMapping, err := s.Network(req.DependenciesEndpoints)
	//if err != nil {
	//	return nil, s.Wool.Wrapf(err, "cannot create network mapping")
	//}
	//
	//nws, err := network.ConvertToEnvironmentVariables(networkMapping)
	//if err != nil {
	//	return nil, s.Wool.Wrapf(err, "cannot convert network mappings")
	//}
	//local := EnvLocal{Envs: nws}
	//// Append Auth0
	//auth0, err := s.GetEnv()
	//if err != nil {
	//	return nil, s.Wool.Wrapf(err, "cannot get env")
	//}
	//local.Envs = append(local.Envs, auth0...)
	//
	//deploy := DeploymentParameter{ConfigMap: EnvsAsMap(local.Envs), Image: s.DockerImage(), Information: s.Information, Deployment: Deployment{Replicas: 1}}
	//err = s.Templates(deploy,
	//	services.WithDeploymentFor(deployment, "kustomize/base", templates.WithOverrideAll()),
	//	services.WithDeploymentFor(deployment, "kustomize/overlays/environment",
	//		services.WithDestination("kustomize/overlays/%s", req.Environment.Name), templates.WithOverrideAll()),
	//)
	//if err != nil {
	//	return nil, err
	//}
	return &factoryv0.DeploymentResponse{}, nil
}

func (s *Factory) Network(es []*basev0.Endpoint) ([]*runtimev0.NetworkMapping, error) {
	//s.DebugMe("in network: %v", configurations.Condensed(es))
	//pm, err := network.NewServiceDnsManager(ctx, s.Identity)
	//if err != nil {
	//	return nil, s.Wool.Wrapf(err, "cannot create network manager")
	//}
	//for _, endpoint := range es {
	//	err = pm.Expose(endpoint)
	//	if err != nil {
	//		return nil, s.Wool.Wrapf(err, "cannot add grpc endpoint to network manager")
	//	}
	//}
	//err = pm.Reserve()
	//if err != nil {
	//	return nil, s.Wool.Wrapf(err, "cannot reserve ports")
	//}
	//return pm.NetworkMapping()
	return nil, nil
}

func (s *Factory) CreateEndpoint(ctx context.Context) error {
	http, err := configurations.NewHTTPApi(ctx, &configurations.Endpoint{Name: "web", Visibility: configurations.VisibilityPublic})
	if err != nil {
		return s.Wool.Wrapf(err, "cannot create HTTP api")
	}
	s.Endpoints = append(s.Endpoints, http)
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

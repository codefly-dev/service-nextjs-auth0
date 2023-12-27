package main

import (
	"context"
	"embed"
	"os"
	"strings"

	dockerhelpers "github.com/codefly-dev/core/agents/helpers/docker"
	"github.com/codefly-dev/core/agents/network"
	"github.com/codefly-dev/core/agents/services"
	"github.com/codefly-dev/core/configurations"
	basev1 "github.com/codefly-dev/core/generated/go/base/v1"
	factoryv1 "github.com/codefly-dev/core/generated/go/services/factory/v1"
	runtimev1 "github.com/codefly-dev/core/generated/go/services/runtime/v1"

	"github.com/codefly-dev/core/runners"
	"github.com/codefly-dev/core/shared"
	"github.com/codefly-dev/core/templates"
)

type Factory struct {
	*Service

	Runner *runners.Runner
}

func NewFactory() *Factory {
	return &Factory{
		Service: NewService(),
	}
}

func (s *Factory) Load(ctx context.Context, req *factoryv1.LoadRequest) (*factoryv1.LoadResponse, error) {
	defer s.Wool.Catch()

	err := s.Factory.Load(ctx, req.Identity, s.Settings)
	if err != nil {
		return nil, err
	}

	gettingStarted, err := templates.ApplyTemplateFrom(shared.Embed(factory), "templates/factory/GETTING_STARTED.md", s.Information)
	if err != nil {
		return nil, err
	}

	return &factoryv1.LoadResponse{
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

func (s *Factory) Create(ctx context.Context, req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer s.Wool.Catch()

	ignores := []string{"node_modules", ".next", ".idea"}

	err := s.Templates(ctx, s.Information, services.WithFactory(factory, ignores...), services.WithBuilder(builder))
	if err != nil {
		return s.Factory.CreateError(err)
	}
	// Need to handle the case of pages/_aps.tsx
	err = templates.Copy(ctx, shared.Embed(special),
		shared.NewFile("templates/special/pages/app.tsx"), shared.NewFile(s.Local("pages/_app.tsx")))
	if err != nil {
		return s.Factory.CreateError(err)
	}

	// out, err := shared.GenerateTree(s.Location, " ")
	// if err != nil {
	// 	return nil, err
	// }
	// s.Wool.Info("tree: %s", out)

	err = os.RemoveAll(s.Local("node_modules"))
	if err != nil {
		return s.Factory.CreateError(err)
	}

	s.Runner = &runners.Runner{
		Name: s.Service.Identity.Name,
		Bin:  "npm",
		Args: []string{"install", "ci"},
		Dir:  s.Location,
	}

	_, err = s.Runner.Run(ctx)
	if err != nil {
		return s.Factory.CreateError(err)
	}

	return s.Factory.CreateResponse(ctx, s.Settings, s.Endpoints...)
}

func (s *Factory) Update(ctx context.Context, req *factoryv1.UpdateRequest) (*factoryv1.UpdateResponse, error) {
	defer s.Wool.Catch()

	return &factoryv1.UpdateResponse{}, nil
}

func (s *Factory) Sync(ctx context.Context, req *factoryv1.SyncRequest) (*factoryv1.SyncResponse, error) {
	defer s.Wool.Catch()

	return &factoryv1.SyncResponse{}, nil
}

type Env struct {
	Key   string
	Value string
}

type DockerTemplating struct {
	Envs []Env
}

func (s *Factory) Build(ctx context.Context, req *factoryv1.BuildRequest) (*factoryv1.BuildResponse, error) {

	s.Wool.Debug("building docker image")

	// We want to use DNS to create NetworkMapping
	networkMapping, err := s.Network(configurations.FlattenEndpoints(ctx, req.DependencyEndpointGroup))
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot create network mapping")
	}

	nws, err := network.ConvertToEnvironmentVariables(networkMapping)
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot convert network mappings")
	}
	local := EnvLocal{Envs: nws}
	// Append Auth0
	auth0, err := s.GetEnv()
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot get env")
	}
	local.Envs = append(local.Envs, auth0...)

	// Generate the .env.local
	err = templates.CopyAndApplyTemplate(ctx, shared.Embed(special),
		shared.NewFile("templates/special/env.local.tmpl"), shared.NewFile(s.Local(".env.local")), local)
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot copy special template")
	}

	err = os.Remove(s.Local("codefly/builder/Dockerfile"))
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot remove dockerfile")
	}
	err = s.Templates(ctx, services.WithBuilder(builder))
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot copy and apply template")
	}
	builder, err := dockerhelpers.NewBuilder(dockerhelpers.BuilderConfiguration{
		Root:       s.Location,
		Dockerfile: "codefly/builder/Dockerfile",
		Image:      s.DockerImage().Name,
		Tag:        s.DockerImage().Tag,
	})
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot create builder")
	}
	// builder.WithLogger(s.Wool)
	_, err = builder.Build(ctx)
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot build image")
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

func (s *Factory) Deploy(ctx context.Context, req *factoryv1.DeploymentRequest) (*factoryv1.DeploymentResponse, error) {
	defer s.Wool.Catch()

	// We want to use DNS to create NetworkMapping
	networkMapping, err := s.Network(configurations.FlattenEndpoints(ctx, req.DependencyEndpointGroup))
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot create network mapping")
	}

	nws, err := network.ConvertToEnvironmentVariables(networkMapping)
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot convert network mappings")
	}
	local := EnvLocal{Envs: nws}
	// Append Auth0
	auth0, err := s.GetEnv()
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot get env")
	}
	local.Envs = append(local.Envs, auth0...)
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
	return &factoryv1.DeploymentResponse{}, nil
}

func (s *Factory) Network(es []*basev1.Endpoint) ([]*runtimev1.NetworkMapping, error) {
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

func (s *Factory) CreateEndpoints() error {

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

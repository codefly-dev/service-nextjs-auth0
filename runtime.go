package main

import (
	"context"
	"os"

	"github.com/codefly-dev/core/configurations"

	"github.com/codefly-dev/core/agents/helpers/code"
	"github.com/codefly-dev/core/agents/network"
	agentv0 "github.com/codefly-dev/core/generated/go/services/agent/v0"
	runtimev0 "github.com/codefly-dev/core/generated/go/services/runtime/v0"
	"github.com/codefly-dev/core/runners"
	"github.com/codefly-dev/core/shared"
	"github.com/codefly-dev/core/templates"
	"github.com/codefly-dev/core/wool"
)

type Runtime struct {
	*Service
	Runner *runners.Runner

	EnvironmentVariables *configurations.EnvironmentVariableManager
}

func NewRuntime() *Runtime {
	return &Runtime{
		Service: NewService(),
	}
}

func (s *Runtime) Load(ctx context.Context, req *runtimev0.LoadRequest) (*runtimev0.LoadResponse, error) {
	defer s.Wool.Catch()

	err := s.Base.Load(ctx, req.Identity, s.Settings)
	if err != nil {
		return s.Base.Runtime.LoadError(err)
	}

	err = s.LoadEndpoints(ctx)
	if err != nil {
		return s.Base.Runtime.LoadError(err)
	}
	return s.Base.Runtime.LoadResponse(s.Endpoints)
}

func (s *Runtime) Init(ctx context.Context, req *runtimev0.InitRequest) (*runtimev0.InitResponse, error) {
	defer s.Wool.Catch()

	s.Wool.Debug("initialize runtime", wool.NullableField("dependency endpoints", configurations.MakeEndpointSummary(req.DependenciesEndpoints)))

	var err error
	s.NetworkMappings, err = s.Network(ctx)
	if err != nil {
		return s.Runtime.InitError(err)
	}

	s.EnvironmentVariables = configurations.NewEnvironmentVariableManager()
	for _, prov := range req.ProviderInfos {
		env := configurations.ProviderInformationAsEnvironmentVariables(prov)
		s.EnvironmentVariables.Add(env...)
	}

	return s.Base.Runtime.InitResponse()
}

func (s *Runtime) Start(ctx context.Context, req *runtimev0.StartRequest) (*runtimev0.StartResponse, error) {
	defer s.Wool.Catch()
	ctx = s.Wool.Inject(ctx)

	s.Wool.Debug("starting runtime", wool.NullableField("network mappings", network.MakeNetworkMappingSummary(req.NetworkMappings)))

	nws, err := network.ConvertToEnvironmentVariables(req.NetworkMappings)
	if err != nil {
		return s.Base.Runtime.StartError(err, wool.InField("converting incoming network mappings"))
	}
	envs := s.EnvironmentVariables.GetBase()

	envs = append(envs, nws...)

	if err != nil {
		return s.Base.Runtime.StartError(err)
	}

	// Generate the .env.local
	s.Wool.Debug("copying special files")
	err = templates.CopyAndApplyTemplate(ctx, shared.Embed(special),
		shared.NewFile("templates/special/env.local.tmpl"),
		shared.NewFile(s.Local(".env.local")),
		envs)
	if err != nil {
		return s.Base.Runtime.StartError(err, wool.InField("copying special files"))
	}

	// Add the group
	s.Runner = &runners.Runner{
		Name: s.Service.Identity.Name,
		Bin:  "npm",
		Args: []string{"run", "dev"},
		Envs: os.Environ(),
		Dir:  s.Location,
	}
	// As usual, create a new context! or we will stop as soon this function returns
	ctx = context.Background()
	ctx = s.Wool.Inject(ctx)
	out, err := s.Runner.Start(ctx)
	if err != nil {
		return s.Base.Runtime.StartError(err, wool.InField("runner"))
	}

	go func() {
		for event := range out.Events {
			s.Wool.Debug("event", wool.Field("event", event))
		}
	}()
	s.Wool.Debug("starting", wool.Field("pid", out.PID))

	return s.Runtime.StartResponse()
}

func (s *Runtime) Information(ctx context.Context, req *runtimev0.InformationRequest) (*runtimev0.InformationResponse, error) {
	return &runtimev0.InformationResponse{}, nil
}

func (s *Runtime) Stop(ctx context.Context, req *runtimev0.StopRequest) (*runtimev0.StopResponse, error) {
	defer s.Wool.Catch()

	s.Wool.Debug("stopping service")
	//err := s.Runner.Kill(ctx)
	//if err != nil {
	//	return nil, s.Wrapf(err, "cannot kill go")
	//}

	err := s.Base.Stop()
	if err != nil {
		return nil, err
	}
	return &runtimev0.StopResponse{}, nil
}

func (s *Runtime) Communicate(ctx context.Context, req *agentv0.Engage) (*agentv0.InformationRequest, error) {
	return s.Base.Communicate(ctx, req)
}

/* Details

 */

func (s *Runtime) EventHandler(event code.Change) error {
	s.Wool.Debug("got an event: %v")
	return nil
}

func (s *Runtime) Network(ctx context.Context) ([]*runtimev0.NetworkMapping, error) {
	pm, err := network.NewServicePortManager(ctx, s.Identity, s.Endpoints...)
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot create default endpoint")
	}
	err = pm.Reserve(ctx)
	if err != nil {
		return nil, s.Wool.Wrapf(err, "cannot reserve ports")
	}
	return pm.NetworkMapping(ctx)
}

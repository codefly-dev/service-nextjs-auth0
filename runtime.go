package main

import (
	"context"
	"github.com/codefly-dev/core/runners"
	"github.com/codefly-dev/core/templates"
	"github.com/codefly-dev/core/wool"
	"os"

	"github.com/codefly-dev/core/agents/helpers/code"
	"github.com/codefly-dev/core/agents/network"
	agentv1 "github.com/codefly-dev/core/generated/go/services/agent/v1"
	runtimev1 "github.com/codefly-dev/core/generated/go/services/runtime/v1"
	"github.com/codefly-dev/core/shared"
)

type Runtime struct {
	*Service
	Runner *runners.Runner

	// internal
}

func NewRuntime() *Runtime {
	return &Runtime{
		Service: NewService(),
	}
}

func (s *Runtime) Load(ctx context.Context, req *runtimev1.LoadRequest) (*runtimev1.LoadResponse, error) {
	defer s.Wool.Catch()

	err := s.Base.Load(ctx, req.Identity, s.Settings)
	if err != nil {
		return s.Base.Runtime.LoadError(err)
	}

	return s.Base.Runtime.LoadResponse(s.Endpoints)
}

func (s *Runtime) Init(ctx context.Context, req *runtimev1.InitRequest) (*runtimev1.InitResponse, error) {
	defer s.Wool.Catch()

	return s.Base.Runtime.InitResponse()
}

type EnvLocal struct {
	Envs []string
}

func (s *Runtime) Start(ctx context.Context, req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer s.Wool.Catch()

	nws, err := network.ConvertToEnvironmentVariables(req.NetworkMappings)
	if err != nil {
		return s.Base.Runtime.StartError(err)
	}
	local := EnvLocal{Envs: nws}
	// Append Auth0
	auth0, err := s.GetEnv()
	if err != nil {
		return s.Base.Runtime.StartError(err)
	}
	local.Envs = append(local.Envs, auth0...)

	// Generate the .env.local
	err = templates.CopyAndApplyTemplate(ctx, shared.Embed(special),
		shared.NewFile("templates/special/env.local.tmpl"), shared.NewFile(s.Local(".env.local")), local)
	if err != nil {
		return s.Base.Runtime.StartError(err)
	}

	// Add the group
	s.Runner = &runners.Runner{
		Name: s.Service.Identity.Name,
		Bin:  "npm",
		Args: []string{"run", "dev"},
		Envs: os.Environ(),
		Dir:  s.Location,
	}
	out, err := s.Runner.Run(ctx)
	if err != nil {
		return s.Base.Runtime.StartError(err)
	}
	tracker := runners.TrackedProcess{PID: out.PID}
	s.Info("starting", wool.Field("pid", out.PID))

	return s.Runtime.StartResponse([]*runtimev1.Tracker{tracker.Proto()})
}

func (s *Runtime) Information(ctx context.Context, req *runtimev1.InformationRequest) (*runtimev1.InformationResponse, error) {
	return &runtimev1.InformationResponse{}, nil
}

func (s *Runtime) Stop(ctx context.Context, req *runtimev1.StopRequest) (*runtimev1.StopResponse, error) {
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
	return &runtimev1.StopResponse{}, nil
}

func (s *Runtime) Communicate(ctx context.Context, req *agentv1.Engage) (*agentv1.InformationRequest, error) {
	return s.Base.Communicate(ctx, req)
}

/* Details

 */

func (s *Runtime) EventHandler(event code.Change) error {
	s.Wool.Debug("got an event: %v")
	return nil
}

func (s *Runtime) Network(ctx context.Context) ([]*runtimev1.NetworkMapping, error) {
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

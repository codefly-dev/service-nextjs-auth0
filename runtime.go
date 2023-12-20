package main

import (
	"context"
	"os"
	"strings"

	"github.com/codefly-dev/core/agents/services"
	"github.com/codefly-dev/core/runners"
	"github.com/codefly-dev/core/templates"

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

func (s *Runtime) Init(ctx context.Context, req *runtimev1.InitRequest) (*runtimev1.InitResponse, error) {
	defer s.Wool.Catch()

	err := s.Base.Init(ctx, req.Identity, s.Settings)
	if err != nil {
		return s.Base.RuntimeInitResponseError(err)
	}

	return s.Base.RuntimeInitResponse(s.Endpoints)
}

func (s *Runtime) Configure(ctx context.Context, req *runtimev1.ConfigureRequest) (*runtimev1.ConfigureResponse, error) {
	defer s.Wool.Catch()

	return &runtimev1.ConfigureResponse{
		Status: services.ConfigureSuccess(),
	}, nil
}

type EnvLocal struct {
	Envs []string
}

func (s *Runtime) Start(ctx context.Context, req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer s.Wool.Catch()

	nws, err := network.ConvertToEnvironmentVariables(req.NetworkMappings)
	if err != nil {
		return nil, s.Wrapf(err, "cannot convert network mappings")
	}
	local := EnvLocal{Envs: nws}
	// Append Auth0
	auth0, err := s.GetEnv()
	if err != nil {
		return nil, s.Wrapf(err, "cannot get env")
	}
	local.Envs = append(local.Envs, auth0...)

	// Generate the .env.local
	err = templates.CopyAndApplyTemplate(ctx, shared.Embed(special),
		shared.NewFile("templates/special/env.local.tmpl"), shared.NewFile(s.Local(".env.local")), local)
	if err != nil {
		return nil, s.Wrapf(err, "cannot copy special template")
	}

	// Add the group
	s.Runner = &runners.Runner{
		Name:  s.Service.Identity.Name,
		Bin:   "npm",
		Args:  []string{"run", "dev"},
		Envs:  os.Environ(),
		Dir:   s.Location,
		Debug: s.Debug,
	}
	err = s.Runner.Init(ctx)
	if err != nil {
		return nil, s.Wrapf(err, "cannot start service")
	}
	//s.Runner.Wait = true
	tracker, err := s.Runner.Run(ctx)
	if err != nil {
		return nil, s.Wrapf(err, "cannot start go program")
	}

	return &runtimev1.StartResponse{
		Status:   services.StartSuccess(),
		Trackers: []*runtimev1.Tracker{tracker.Proto()},
	}, nil
}

func (s *Runtime) Information(ctx context.Context, req *runtimev1.InformationRequest) (*runtimev1.InformationResponse, error) {
	return &runtimev1.InformationResponse{}, nil
}

func (s *Runtime) Stop(ctx context.Context, req *runtimev1.StopRequest) (*runtimev1.StopResponse, error) {
	defer s.Wool.Catch()

	s.Wool.Debug("stopping service")
	err := s.Runner.Kill(ctx)
	if err != nil {
		return nil, s.Wrapf(err, "cannot kill go")
	}

	err = s.Base.Stop()
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
	if strings.Contains(event.Path, "proto") {
		s.WantSync()
	} else {
		s.WantRestart()
	}
	err := s.Runner.Init(context.Background())
	if err != nil {
		// s.ServiceLogger.Info("Detected code changes: still cannot restart: %v", err)
		return err
	}
	// s.ServiceLogger.Info("Detected code changes: restarting")
	return nil
}

func (s *Runtime) Network(ctx context.Context) ([]*runtimev1.NetworkMapping, error) {
	pm, err := network.NewServicePortManager(ctx, s.Identity, s.Endpoints...)
	if err != nil {
		return nil, s.Wrapf(err, "cannot create default endpoint")
	}
	err = pm.Reserve(ctx)
	if err != nil {
		return nil, s.Wrapf(err, "cannot reserve ports")
	}
	return pm.NetworkMapping(ctx)
}

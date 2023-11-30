package main

import (
	"context"
	"github.com/codefly-dev/core/agents/services"
	agentsv1 "github.com/codefly-dev/core/proto/v1/go/agents"
	"github.com/codefly-dev/core/runners"
	"github.com/codefly-dev/core/templates"
	"os"
	"strings"

	"github.com/codefly-dev/core/agents/helpers/code"
	"github.com/codefly-dev/core/agents/network"
	servicev1 "github.com/codefly-dev/core/proto/v1/go/services"
	runtimev1 "github.com/codefly-dev/core/proto/v1/go/services/runtime"
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

func (p *Runtime) Init(req *servicev1.InitRequest) (*runtimev1.InitResponse, error) {
	defer p.AgentLogger.Catch()

	err := p.Base.Init(req, p.Settings)
	if err != nil {
		return p.Base.RuntimeInitResponseError(err)
	}

	return p.Base.RuntimeInitResponse(p.Endpoints)
}

func (p *Runtime) Configure(req *runtimev1.ConfigureRequest) (*runtimev1.ConfigureResponse, error) {
	defer p.AgentLogger.Catch()

	p.ServiceLogger.Info("watching code changes")

	return &runtimev1.ConfigureResponse{
		Status: services.ConfigureSuccess(),
	}, nil
}

type EnvLocal struct {
	Envs []string
}

func (p *Runtime) GetEnv() ([]string, error) {
	// read the env file for auth0
	f, err := os.ReadFile(p.Local("auth0.env"))
	if err != nil {
		return nil, p.Wrapf(err, "cannot read auth0.env")
	}
	envs := strings.Split(string(f), "\n")
	return envs, nil
}

func (p *Runtime) Start(req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer p.AgentLogger.Catch()

	nws, err := network.ConvertToEnvironmentVariables(req.NetworkMappings)
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

	// Generate the .env.local
	err = templates.CopyAndApplyTemplate(shared.Embed(special),
		shared.NewFile("templates/special/env.local.tmpl"), shared.NewFile(p.Local(".env.local")), local)
	if err != nil {
		return nil, p.Wrapf(err, "cannot copy special template")
	}

	// Add the group
	p.Runner = &runners.Runner{
		Name:          p.Service.Identity.Name,
		Bin:           "npm",
		Args:          []string{"run", "dev"},
		Envs:          os.Environ(),
		AgentLogger:   p.AgentLogger,
		ServiceLogger: p.ServiceLogger,
		Dir:           p.Location,
		Debug:         p.Debug,
	}
	err = p.Runner.Init(p.Context())
	if err != nil {
		return nil, p.Wrapf(err, "cannot start service")
	}
	//p.Runner.Wait = true
	tracker, err := p.Runner.Run(p.Context())
	if err != nil {
		return nil, p.Wrapf(err, "cannot start go program")
	}

	return &runtimev1.StartResponse{
		Status:   services.StartSuccess(),
		Trackers: []*runtimev1.Tracker{tracker.Proto()},
	}, nil
}

func (p *Runtime) Information(req *runtimev1.InformationRequest) (*runtimev1.InformationResponse, error) {
	return &runtimev1.InformationResponse{}, nil
}

func (p *Runtime) Stop(req *runtimev1.StopRequest) (*runtimev1.StopResponse, error) {
	defer p.AgentLogger.Catch()

	p.AgentLogger.Debugf("stopping service")
	err := p.Runner.Kill()
	if err != nil {
		return nil, shared.Wrapf(err, "cannot kill go")
	}

	err = p.Base.Stop()
	if err != nil {
		return nil, err
	}
	return &runtimev1.StopResponse{}, nil
}

func (p *Runtime) Communicate(req *agentsv1.Engage) (*agentsv1.InformationRequest, error) {
	return p.Base.Communicate(req)
}

/* Details

 */

func (p *Runtime) EventHandler(event code.Change) error {
	p.AgentLogger.Debugf("got an event: %v", event)
	if strings.Contains(event.Path, "proto") {
		p.WantSync()
	} else {
		p.WantRestart()
	}
	err := p.Runner.Init(context.Background())
	if err != nil {
		p.ServiceLogger.Info("Detected code changes: still cannot restart: %v", err)
		return err
	}
	p.ServiceLogger.Info("Detected code changes: restarting")
	return nil
}

func (p *Runtime) Network() ([]*runtimev1.NetworkMapping, error) {
	pm, err := network.NewServicePortManager(p.Context(), p.Identity, p.Endpoints...)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot create default endpoint")
	}
	err = pm.Reserve()
	if err != nil {
		return nil, shared.Wrapf(err, "cannot reserve ports")
	}
	return pm.NetworkMapping()
}

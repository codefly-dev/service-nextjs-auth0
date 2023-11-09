package main

import (
	"context"
	"os"
	"strings"

	"github.com/codefly-dev/cli/pkg/plugins"
	"github.com/codefly-dev/cli/pkg/plugins/endpoints"
	"github.com/codefly-dev/cli/pkg/plugins/helpers/code"
	dockerhelpers "github.com/codefly-dev/cli/pkg/plugins/helpers/docker"
	golanghelpers "github.com/codefly-dev/cli/pkg/plugins/helpers/go"
	"github.com/codefly-dev/cli/pkg/plugins/network"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	corev1 "github.com/codefly-dev/cli/proto/v1/core"
	v1 "github.com/codefly-dev/cli/proto/v1/services"
	runtimev1 "github.com/codefly-dev/cli/proto/v1/services/runtime"
	"github.com/codefly-dev/core/configurations"
	"github.com/codefly-dev/core/shared"
	"github.com/pkg/errors"
)

type Runtime struct {
	*Service

	// Endpoints
	GrpcEndpoint *corev1.Endpoint
	RestEndpoint *corev1.Endpoint

	// internal
	Runner *golanghelpers.Runner
}

func NewRuntime() *Runtime {
	return &Runtime{
		Service: NewService(),
	}
}

func (p *Runtime) Init(req *v1.InitRequest) (*runtimev1.InitResponse, error) {
	defer p.PluginLogger.Catch()

	err := p.Base.Init(req)
	if err != nil {
		return nil, err
	}

	err = p.LoadEndpoints()
	if err != nil {
		return nil, err
	}

	return p.Base.RuntimeInitResponse(p.Endpoints)
}

func (p *Runtime) Configure(req *runtimev1.ConfigureRequest) (*runtimev1.ConfigureResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.TODO("refactor events")

	nets, err := p.Network()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create default endpoint")
	}

	p.Runner = &golanghelpers.Runner{
		Dir:           p.Location,
		Args:          []string{"main.go"},
		ServiceLogger: plugins.NewServiceLogger(p.Identity.Name),
		PluginLogger:  p.PluginLogger,
		Envs:          network.ConvertToEnvironmentVariables(nets),
		Debug:         p.Spec.Debug,
	}

	if p.Spec.Watch {
		conf := services.NewWatchConfiguration([]string{".", "adapters"}, "service.codefly.yaml")
		err := p.SetupWatcher(conf, p.EventHandler)
		if err != nil {
			p.PluginLogger.Warn("error in watcher")
		}
	}

	err = p.Runner.Init(context.Background())
	if err != nil {
		p.ServiceLogger.Info("-> Cannot init: %v", err)
		return &runtimev1.ConfigureResponse{Status: services.ConfigureError(err)}, nil
	}

	return &runtimev1.ConfigureResponse{
		Status:          services.ConfigureSuccess(),
		NetworkMappings: nets,
	}, nil
}

func (p *Runtime) Start(req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer p.PluginLogger.Catch()

	ctx := context.Background()

	p.PluginLogger.Debugf("network mapping: %v", req.NetworkMappings)

	p.Runner.Envs = append(p.Runner.Envs, network.ConvertToEnvironmentVariables(req.NetworkMappings)...)

	tracker, err := p.Runner.Run(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot run go")
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
	defer p.PluginLogger.Catch()

	p.PluginLogger.Debugf("stopping service")
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

func (p *Runtime) Sync(req *runtimev1.SyncRequest) (*runtimev1.SyncResponse, error) {
	defer p.PluginLogger.Catch()

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

	return &runtimev1.SyncResponse{}, nil
}

func (p *Runtime) Build(req *runtimev1.BuildRequest) (*runtimev1.BuildResponse, error) {
	p.PluginLogger.Debugf("building docker image")
	builder, err := dockerhelpers.NewBuilder(dockerhelpers.BuilderConfiguration{
		Root:  p.Location,
		Image: p.Identity.Name,
		Tag:   p.Configuration.Version,
	})
	if err != nil {
		return nil, p.Wrapf(err, "cannot create builder")
	}
	builder.WithLogger(p.PluginLogger)
	_, err = builder.Build()
	if err != nil {
		return nil, p.Wrapf(err, "cannot build image")
	}
	return &runtimev1.BuildResponse{}, nil
}

func (p *Runtime) Deploy(req *runtimev1.DeploymentRequest) (*runtimev1.DeploymentResponse, error) {
	return &runtimev1.DeploymentResponse{}, nil
}

func (p *Runtime) Communicate(req *corev1.Engage) (*corev1.InformationRequest, error) {
	return p.Base.Communicate(req)
}

/* Details

 */

func (p *Runtime) EventHandler(event code.Change) error {
	p.PluginLogger.Debugf("got an event: %v", event)
	if strings.Contains(event.Path, "proto") {
		p.ServiceLogger.Info("Detected proto changes: will sync dependencies")
		_, err := p.Sync(&runtimev1.SyncRequest{})
		if err != nil {
			p.PluginLogger.Warn("cannot sync proto: %v", err)
		}
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
	pm := network.NewServicePortManager(p.Identity, p.Endpoints...).WithHost("localhost").WithLogger(p.PluginLogger)
	err := pm.Expose(p.GrpcEndpoint)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot add grpc endpoint to network manager")
	}
	if p.RestEndpoint != nil {
		err = pm.Expose(p.RestEndpoint)
		if err != nil {
			return nil, shared.Wrapf(err, "cannot add rest to network manager")
		}
	}
	err = pm.Reserve()
	if err != nil {
		return nil, shared.Wrapf(err, "cannot reserve ports")
	}
	return pm.NetworkMapping()
}

func (p *Runtime) LoadEndpoints() error {
	var err error
	for _, ep := range p.Configuration.Endpoints {
		switch ep.Api.Protocol {
		case configurations.Grpc:
			p.GrpcEndpoint, err = endpoints.NewGrpcApi(ep, p.Local("api.proto"))
			if err != nil {
				return p.Wrapf(err, "cannot create grpc api")
			}
			p.Endpoints = append(p.Endpoints, p.GrpcEndpoint)
		case configurations.Rest:
			p.RestEndpoint, err = endpoints.NewRestApiFromOpenAPI(p.Context(), ep, p.Local("api.swagger.json"))
			if err != nil {
				return p.Wrapf(err, "cannot create openapi api")
			}
			p.Endpoints = append(p.Endpoints, p.RestEndpoint)
		}
	}
	return nil
}

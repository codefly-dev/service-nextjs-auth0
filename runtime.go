package main

import (
	"context"
	"github.com/codefly-dev/cli/pkg/plugins"
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
	"strings"
)

type Runtime struct {
	*Service

	// internal
	Runner *golanghelpers.Runner
}

func NewRuntime() *Runtime {
	return &Runtime{
		Service: NewService(),
	}
}

func (p *Runtime) Init(req *v1.InitRequest) (*runtimev1.InitResponse, error) {
	defer p.Base.PluginLogger.Catch()

	err := p.Base.Init(req, &p.Spec)
	if err != nil {
		return nil, err
	}
	p.HydrateEndpoints()

	grpc, err := services.NewGrpcApi(p.Base.Local("api.proto"))
	if err != nil {
		return nil, shared.Wrapf(err, "cannot create grpc api")
	}
	endpoint, err := services.WithApi(&p.GrpcEndpoint, grpc)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot add gRPC api to endpoint")
	}
	endpoints := []*corev1.Endpoint{endpoint}

	if p.RestEndpoint != nil {
		rest, err := services.NewOpenApi(p.Base.Local("adapters/v1/swagger/api.swagger.json"))
		if err != nil {
			return nil, shared.Wrapf(err, "cannot create REST api")
		}
		r, err := services.WithApi(p.RestEndpoint, rest)
		if err != nil {
			return nil, shared.Wrapf(err, "cannot add grpc api to endpoint")
		}
		endpoints = append(endpoints, r)
	}
	return &runtimev1.InitResponse{
		Version:   p.Base.Version(),
		Endpoints: endpoints,
	}, nil

}

func (p *Runtime) Configure(req *runtimev1.ConfigureRequest) (*runtimev1.ConfigureResponse, error) {
	defer p.Base.PluginLogger.Catch()

	p.Base.PluginLogger.TODO("refactor events")

	p.Runner = &golanghelpers.Runner{
		Dir:           p.Base.Location,
		Args:          []string{"main.go"},
		ServiceLogger: plugins.NewServiceLogger(p.Base.Identity.Name),
		PluginLogger:  p.Base.PluginLogger,
		Debug:         p.Spec.Debug,
	}

	if p.Spec.Watch {
		conf := services.NewWatchConfiguration([]string{".", "adapters"}, "service.codefly.yaml")
		err := p.Base.SetupWatcher(conf, p.EventHandler)
		if err != nil {
			p.Base.PluginLogger.Warn("error in watcher")
		}
	}

	err := p.Runner.Init(context.Background())
	if err != nil {
		p.Base.ServiceLogger.Info("-> Cannot init: %v", err)
		return &runtimev1.ConfigureResponse{Status: services.InitError(err)}, nil
	}

	nets, err := p.Network()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create default endpoint")
	}

	return &runtimev1.ConfigureResponse{
		Status:          services.InitReady(),
		NetworkMappings: nets,
	}, nil
}

func (p *Runtime) Start(req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer p.Base.PluginLogger.Catch()

	ctx := context.Background()

	p.Base.PluginLogger.Info("network mapping: %v", req.NetworkMappings)

	p.Runner.Envs = network.ConvertToEnvironmentVariables(req.NetworkMappings)

	tracker, err := p.Runner.Run(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot run go")
	}

	return &runtimev1.StartResponse{
		Status:   p.Base.StartSuccess(),
		Trackers: []*runtimev1.Tracker{tracker.Proto()},
	}, nil
}

func (p *Runtime) Information(req *runtimev1.InformationRequest) (*runtimev1.InformationResponse, error) {
	return &runtimev1.InformationResponse{Status: p.Base.Status}, nil
}

func (p *Runtime) Stop(req *runtimev1.StopRequest) (*runtimev1.StopResponse, error) {
	defer p.Base.PluginLogger.Catch()

	p.Base.PluginLogger.Debugf("stopping service")
	err := p.Runner.Kill()
	if err != nil {
		return nil, shared.Wrapf(err, "cannot kill go")
	}

	p.Base.Stop()
	return &runtimev1.StopResponse{}, nil
}

func (p *Runtime) Sync(req *runtimev1.SyncRequest) (*runtimev1.SyncResponse, error) {
	defer p.Base.PluginLogger.Catch()

	p.Base.PluginLogger.Debugf("running sync: %v", p.Base.Location)
	helper := golanghelpers.Go{Dir: p.Base.Location}
	err := helper.ModTidy(p.Base.PluginLogger)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot tidy go.mod")
	}
	err = helper.BufGenerate(p.Base.PluginLogger)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot generate proto")
	}
	return &runtimev1.SyncResponse{}, nil
}

func (p *Runtime) Build(req *runtimev1.BuildRequest) (*runtimev1.BuildResponse, error) {
	p.Base.PluginLogger.Debugf("building docker image")
	builder, err := dockerhelpers.NewBuilder(dockerhelpers.BuilderConfiguration{
		Root:  p.Base.Location,
		Image: p.Base.Identity.Name,
		Tag:   p.Base.Configuration.Version,
	})
	if err != nil {
		return nil, p.Base.PluginLogger.Wrapf(err, "cannot create builder")
	}
	builder.WithLogger(p.Base.PluginLogger)
	_, err = builder.Build()
	if err != nil {
		return nil, p.Base.PluginLogger.Wrapf(err, "cannot build image")
	}
	return &runtimev1.BuildResponse{}, nil
}

func (p *Runtime) Deploy(req *runtimev1.DeploymentRequest) (*runtimev1.DeploymentResponse, error) {
	return &runtimev1.DeploymentResponse{}, nil
}

func (p *Runtime) Communicate(req *corev1.Question) (*corev1.Answer, error) {
	panic("implement me")
}

/* Details

 */

func (p *Runtime) EventHandler(event code.Change) error {
	p.Base.PluginLogger.DebugMe("got an event: %v", event)
	if strings.Contains(event.Path, "proto") {
		_, err := p.Sync(&runtimev1.SyncRequest{})
		if err != nil {
			p.Base.PluginLogger.Warn("cannot sync proto: %v", err)
		}
	}
	err := p.Runner.Init(context.Background())
	if err != nil {
		p.Base.ServiceLogger.Info("-> Detected code changes: still cannot restart: %v", err)
		return err
	}
	p.Base.ServiceLogger.Info("-> Detected working code changes: restarting")
	p.Base.PluginLogger.DebugMe("detected working code changes: restarting")
	p.Base.WantRestart()
	return nil
}

func (p *Runtime) Network() ([]*runtimev1.NetworkMapping, error) {
	endpoints := []configurations.Endpoint{p.GrpcEndpoint}
	if p.RestEndpoint != nil {
		endpoints = append(endpoints, *p.RestEndpoint)
	}
	pm := network.NewServicePortManager(p.Base.Identity, endpoints...).WithHost("localhost").WithLogger(p.Base.PluginLogger)
	err := pm.Expose(p.GrpcEndpoint, network.Grpc())
	if err != nil {
		return nil, shared.Wrapf(err, "cannot add grpc endpoint to network manager")
	}
	if p.RestEndpoint != nil {
		err = pm.Expose(*p.RestEndpoint, network.Http())
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

func (p *Runtime) HydrateEndpoints() {
	for _, ep := range p.Base.Configuration.Endpoints {
		switch ep.Api.Protocol {
		case configurations.Grpc:
			p.GrpcEndpoint = configurations.Endpoint{
				Name:        configurations.Grpc,
				Api:         ep.Api,
				Public:      ep.Public,
				Description: ep.Description,
			}
		case configurations.Http:
			p.RestEndpoint = &configurations.Endpoint{
				Name:        configurations.Http,
				Api:         ep.Api,
				Public:      ep.Public,
				Description: ep.Description,
			}
		}

	}
}

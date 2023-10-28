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
	runtimev1 "github.com/codefly-dev/cli/proto/v1/services/runtime"
	"github.com/codefly-dev/core/configurations"
	"github.com/codefly-dev/core/shared"
	"github.com/pkg/errors"
	"path"
	"strings"
	"sync"
)

type Runtime struct {
	*Service
	Identity *runtimev1.ServiceIdentity

	ServiceLogger *plugins.ServiceLogger

	// internal
	Runner *golanghelpers.Runner
	status services.InformationStatus

	mutex         *sync.Mutex
	events        chan code.Change
	watcher       *code.Watcher
	Configuration *configurations.Service
}

func NewRuntime() *Runtime {
	return &Runtime{
		Service: NewService(),
		mutex:   &sync.Mutex{},
	}
}

func (p *Runtime) Local(f string) string {
	return path.Join(p.Location, f)
}

func (p *Runtime) Configure(req *runtimev1.ConfigureRequest) (*runtimev1.ConfigureResponse, error) {
	defer p.PluginLogger.Catch()

	err := configurations.LoadSpec(req.Spec, &p.Spec, shared.BaseLogger(p.PluginLogger))
	if err != nil {
		return nil, errors.Wrapf(err, "factory[init]: cannot load spec")
	}

	if req.Debug {
		p.PluginLogger.SetDebug() // For developers
	}

	p.Location = req.Location
	p.Identity = req.Identity

	p.Configuration, err = configurations.LoadFromDir[configurations.Service](p.Location)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot load service configuration")
	}
	p.ServiceLogger = plugins.NewServiceLogger(p.Identity.Name)

	p.HydrateEndpoints()

	grpc, err := services.NewGrpcApi(p.Local("api.proto"))
	if err != nil {
		return nil, shared.Wrapf(err, "cannot create grpc api")
	}
	endpoint, err := services.WithApi(&p.GrpcEndpoint, grpc)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot add gRPC api to endpoint")
	}
	endpoints := []*corev1.Endpoint{endpoint}

	if p.RestEndpoint != nil {
		rest, err := services.NewOpenApi(p.Local("adapters/v1/swagger/api.swagger.json"))
		if err != nil {
			return nil, shared.Wrapf(err, "cannot create REST api")
		}
		r, err := services.WithApi(p.RestEndpoint, rest)
		if err != nil {
			return nil, shared.Wrapf(err, "cannot add grpc api to endpoint")
		}
		endpoints = append(endpoints, r)
	}

	return &runtimev1.ConfigureResponse{Endpoints: endpoints}, nil
}

func (p *Runtime) Init(req *runtimev1.InitRequest) (*runtimev1.InitResponse, error) {
	defer p.PluginLogger.Catch()

	p.status = services.Init

	p.PluginLogger.TODO("refactor events")
	p.PluginLogger.Debugf("creating event channel")
	p.events = make(chan code.Change)

	p.Runner = &golanghelpers.Runner{
		Dir:           p.Location,
		Args:          []string{"main.go"},
		ServiceLogger: plugins.NewServiceLogger(p.Identity.Name),
		PluginLogger:  p.PluginLogger,
		Debug:         p.Spec.Debug,
	}

	if p.Spec.Watch {
		p.PluginLogger.Debugf("watching for code changes")
		err := p.setupWatcher()
		if err != nil {
			p.PluginLogger.Warn("error in watcher")
		}
		p.ServiceLogger.Info("-> Watching for code changes")
	}

	err := p.Runner.Init(context.Background())
	if err != nil {
		p.ServiceLogger.Info("-> Cannot init: %v", err)
		return &runtimev1.InitResponse{Status: &runtimev1.InitStatus{
			Status:  runtimev1.InitStatus_ERROR,
			Message: err.Error(),
		}}, nil
	}

	nets, err := p.Network()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create default endpoint")
	}

	return &runtimev1.InitResponse{
		NetworkMappings: nets,
		Status: &runtimev1.InitStatus{
			Status: runtimev1.InitStatus_READY,
		},
	}, nil
}

func (p *Runtime) Start(req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer p.PluginLogger.Catch()

	ctx := context.Background()

	p.PluginLogger.Info("%s: network mapping: %v", p.Identity.Name, req.NetworkMappings)

	p.Runner.Envs = network.ConvertToEnvironmentVariables(req.NetworkMappings)

	tracker, err := p.Runner.Run(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot run go")
	}

	p.status = services.Started
	return &runtimev1.StartResponse{
		Status: &runtimev1.StartStatus{
			Status: runtimev1.StartStatus_STARTED,
		},
		Trackers: []*runtimev1.Tracker{tracker.Proto()},
	}, nil
}

func (p *Runtime) Information(req *runtimev1.InformationRequest) (*runtimev1.InformationResponse, error) {
	return &runtimev1.InformationResponse{Status: p.status}, nil
}

func (p *Runtime) Stop(req *runtimev1.StopRequest) (*runtimev1.StopResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.Debugf("%s: stopping service", p.Identity.Name)
	err := p.Runner.Kill()
	if err != nil {
		return nil, shared.Wrapf(err, "cannot kill go")
	}

	p.status = services.Stopped
	close(p.events)
	return &runtimev1.StopResponse{}, nil
}

func (p *Runtime) Sync(req *runtimev1.SyncRequest) (*runtimev1.SyncResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.Debugf("running sync: %v", p.Location)
	helper := golanghelpers.Go{Dir: p.Location}
	err := helper.ModTidy(p.PluginLogger)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot tidy go.mod")
	}
	err = helper.BufGenerate(p.PluginLogger)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot generate proto")
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
		return nil, p.PluginLogger.Wrapf(err, "cannot create builder")
	}
	builder.WithLogger(p.PluginLogger)
	_, err = builder.Build()
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot build image")
	}
	return &runtimev1.BuildResponse{}, nil
}

func (p *Runtime) Deploy(req *runtimev1.DeploymentRequest) (*runtimev1.DeploymentResponse, error) {
	return &runtimev1.DeploymentResponse{}, nil
}

func (p *Runtime) Communicate(req *corev1.Question) (*corev1.Answer, error) {
	//TODO implement me
	panic("implement me")
}

/* Details

 */

func (p *Runtime) setupWatcher() error {
	p.PluginLogger.DebugMe("%s: watching for changes", p.Identity.Name)
	var err error
	p.watcher, err = code.NewWatcher(p.PluginLogger, p.events, p.Location, []string{".", "adapters"}, "service.codefly.yaml")
	if err != nil {
		return err
	}
	go p.watcher.Start()

	go func() {
		for event := range p.events {
			p.PluginLogger.DebugMe("got an event: %v", event)
			if strings.Contains(event.Path, "proto") {
				_, err := p.Sync(&runtimev1.SyncRequest{})
				if err != nil {
					p.PluginLogger.Warn("cannot sync proto: %v", err)
				}
			}
			err := p.Runner.Init(context.Background())
			if err != nil {
				p.ServiceLogger.Info("-> Detected code changes: still cannot restart: %v", err)
				continue
			}
			p.ServiceLogger.Info("-> Detected working code changes: restarting")
			p.PluginLogger.DebugMe("detected working code changes: restarting")
			p.mutex.Lock()
			p.status = services.RestartWanted
			p.mutex.Unlock()
		}
	}()
	return nil
}

func (p *Runtime) Network() ([]*runtimev1.NetworkMapping, error) {
	endpoints := []configurations.Endpoint{p.GrpcEndpoint}
	if p.RestEndpoint != nil {
		endpoints = append(endpoints, *p.RestEndpoint)
	}
	pm := network.NewServicePortManager(p.Identity, endpoints...).WithHost("localhost").WithLogger(p.PluginLogger)
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
	for _, ep := range p.Configuration.Endpoints {
		switch ep.Api.Protocol {
		case configurations.Grpc:
			p.GrpcEndpoint = configurations.Endpoint{
				Name:        configurations.Grpc,
				Api:         ep.Api,
				Public:      ep.Endpoint.Public,
				Description: ep.Endpoint.Description,
			}
		case configurations.Http:
			p.RestEndpoint = &configurations.Endpoint{
				Name:        configurations.Http,
				Api:         ep.Api,
				Public:      ep.Endpoint.Public,
				Description: ep.Endpoint.Description,
			}
		}

	}
}

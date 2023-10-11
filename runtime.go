package main

import (
	"context"
	"github.com/hygge-io/hygge/pkg/configurations"
	"github.com/hygge-io/hygge/pkg/core"
	"github.com/hygge-io/hygge/pkg/plugins"
	"github.com/hygge-io/hygge/pkg/plugins/helpers/code"
	golanghelpers "github.com/hygge-io/hygge/pkg/plugins/helpers/go"
	"github.com/hygge-io/hygge/pkg/plugins/network"
	"github.com/hygge-io/hygge/pkg/plugins/services"
	runtimev1 "github.com/hygge-io/hygge/proto/v1/services/runtime"
	"github.com/pkg/errors"
	"path"
	"strings"
	"sync"
)

type Runtime struct {
	*Service
	Identity *runtimev1.ServiceIdentity

	// internal
	Runner *golanghelpers.Runner
	status services.InformationStatus
	sync.Mutex
}

func NewRuntime() *Runtime {
	return &Runtime{
		Service: NewService(),
	}
}

func (p *Runtime) Local(f string) string {
	return path.Join(p.Location, f)
}

func (p *Runtime) Configure(req *runtimev1.ConfigureRequest) (*runtimev1.ConfigureResponse, error) {
	defer p.PluginLogger.Catch()

	err := configurations.LoadSpec(req.Spec, &p.Spec, core.BaseLogger(p.PluginLogger))
	if err != nil {
		return nil, errors.Wrapf(err, "factory[init]: cannot load spec")
	}

	p.PluginLogger.SetDebug(req.Debug) // For developers

	p.Location = req.Location
	p.Identity = req.Identity
	p.InitEndpoints()

	p.PluginLogger.Info("runtime[init] initializing p at <%s> with Spec: %v", req.Location, p.Spec)

	grpc, err := services.NewGrpcApi(p.Local("api.proto"))
	if err != nil {
		return nil, core.Wrapf(err, "cannot create grpc api")
	}
	endpoints, err := services.WithApis(grpc, p.GrpcEndpoint)
	if err != nil {
		return nil, core.Wrapf(err, "cannot add gRPC api to endpoint")
	}

	if p.Spec.CreateHttpEndpoint {
		rest, err := services.NewOpenApi(p.Local("adapters/v1/swagger/api.swagger.json"))
		if err != nil {
			return nil, core.Wrapf(err, "cannot create REST api")
		}
		other, err := services.WithApis(rest, *p.RestEndpoint)
		if err != nil {
			return nil, core.Wrapf(err, "cannot add grpc api to endpoint")
		}
		endpoints = append(endpoints, other...)
	}

	return &runtimev1.ConfigureResponse{Endpoints: endpoints}, nil
}

func (p *Runtime) Init(req *runtimev1.InitRequest) (*runtimev1.InitResponse, error) {
	defer p.PluginLogger.Catch()

	p.status = services.Init

	nets, err := p.Network()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create default endpoint")
	}
	return &runtimev1.InitResponse{NetworkMappings: nets}, nil
}

func (p *Runtime) Start(req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.Info("runtime[starting] go program in <%s> with spec: %v", p.Location, p.Spec)

	events := make(chan code.Change)

	if p.Spec.Watch {
		err := p.setupWatcher(events)
		if err != nil {
			return nil, core.Wrapf(err, "cannot setup watcher")
		}
	}

	env, err := plugins.GetEnvironment(req.Environment)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get environment")
	}
	p.PluginLogger.DebugMe("ENV: %v", env.Variables)

	p.Runner = &golanghelpers.Runner{
		Dir:           p.Location,
		Args:          []string{"main.go"},
		Envs:          env.Variables,
		ServiceLogger: plugins.NewServiceLogger(p.Identity.Name),
		PluginLogger:  p.PluginLogger,
		Debug:         p.Spec.Debug,
	}
	tracker, err := p.Runner.Run(context.Background())
	if err != nil {
		return nil, errors.Wrapf(err, "cannot run go")
	}

	p.status = services.Started
	return &runtimev1.StartResponse{
		Trackers: []*runtimev1.Tracker{tracker.Proto()},
	}, nil
}

func (p *Runtime) Information(req *runtimev1.InformationRequest) (*runtimev1.InformationResponse, error) {
	return &runtimev1.InformationResponse{Status: p.status}, nil
}

func (p *Runtime) Stop(req *runtimev1.StopRequest) (*runtimev1.StopResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.Debugf("runtime[stop] stopping service at <%s>", p.Location)
	_ = p.Runner.Kill()

	p.status = services.Stopped
	return &runtimev1.StopResponse{}, nil
}

func (p *Runtime) Sync(req *runtimev1.SyncRequest) (*runtimev1.SyncResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.Debugf("running sync: %v", p.Location)
	helper := golanghelpers.Go{Dir: p.Location}
	err := helper.ModTidy(p.PluginLogger)
	if err != nil {
		return nil, core.Wrapf(err, "cannot tidy go.mod")
	}
	err = helper.BufGenerate(p.PluginLogger)
	if err != nil {
		return nil, core.Wrapf(err, "cannot generate proto")
	}
	return &runtimev1.SyncResponse{}, nil
}

func (p *Runtime) Build(req *runtimev1.BuildRequest) (*runtimev1.BuildResponse, error) {
	return &runtimev1.BuildResponse{}, nil
}

func (p *Runtime) Deploy(req *runtimev1.DeploymentRequest) (*runtimev1.DeploymentResponse, error) {
	return &runtimev1.DeploymentResponse{}, nil
}

/* Details

 */

func (p *Runtime) setupWatcher(events chan code.Change) error {
	p.PluginLogger.Info("runtime[starting] watching for changes in <%s>", p.Location)
	_, err := code.NewWatcher(p.PluginLogger, events, p.Location, []string{"."})
	if err != nil {
		return err
	}

	go func() {
		for event := range events {
			if strings.Contains(event.Path, "proto") {
				p.PluginLogger.Info("runtime[starting] proto change detected: %v | DO NOTHING FOR NOW", event)
				continue
			}
			p.PluginLogger.Info("runtime[starting] relevant changes to go program detected: killing")
			// Probably should lock
			p.Lock()
			p.status = services.RestartWanted
			p.Unlock()
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
		return nil, core.Wrapf(err, "cannot add grpc endpoint to network manager")
	}
	if p.RestEndpoint != nil {
		err = pm.Expose(*p.RestEndpoint, network.Http())
		if err != nil {
			return nil, core.Wrapf(err, "cannot add rest to network manager")
		}
	}
	err = pm.Reserve()
	if err != nil {
		return nil, core.Wrapf(err, "cannot reserve ports")
	}
	return pm.NetworkMapping()
}

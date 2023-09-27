package main

import (
	"context"
	"github.com/hygge-io/hygge/pkg/configurations"
	"github.com/hygge-io/hygge/pkg/core"
	"github.com/hygge-io/hygge/pkg/plugins"
	"github.com/hygge-io/hygge/pkg/plugins/helpers/code"
	golanghelpers "github.com/hygge-io/hygge/pkg/plugins/helpers/go"
	"github.com/hygge-io/hygge/pkg/plugins/services"
	runtimev1 "github.com/hygge-io/hygge/proto/v1/services/runtime"
	"path"
	"strings"
)

type Runtime struct {
	*Service
	Name   string
	Runner *golanghelpers.Run
	// internal
	status services.ServiceInformationStatus
}

func NewRuntime() *Runtime {
	return &Runtime{
		Service: NewService(),
	}
}

func (p *Runtime) Init(req *runtimev1.InitRequest) (*runtimev1.InitResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.SetDebug(req.Debug) // For developers
	p.Location = req.Location
	p.Name = req.Identity.Name

	p.status = services.Init

	err := configurations.LoadSpec(req.Spec, &p.Spec, core.BaseLogger(p.PluginLogger))
	if err != nil {
		return nil, p.PluginLogger.WrapErrorf(err, "factory[init]: cannot load spec")
	}
	p.PluginLogger.Info("runtime[init] initializing p at <%s> with Spec: %v", req.Location, p.Spec)

	return &runtimev1.InitResponse{}, nil
}

func (p *Runtime) Start(req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer p.PluginLogger.Catch()

	dir := path.Join(p.Location, Source)
	p.PluginLogger.Info("runtime[starting] go program in <%s> with spec: %v", dir, p.Spec)

	helper := golanghelpers.Go{Dir: path.Join(p.Location, Source)}

	// Exclude the proto
	events := make(chan code.Change)
	if p.Spec.Watch {
		p.PluginLogger.Info("runtime[starting] watching for changes in <%s>", dir)
		_, err := code.NewWatcher(p.PluginLogger, events, dir, []string{"."}, "api.proto")
		if err != nil {
			return nil, p.PluginLogger.WrapErrorf(err, "cannot create code watcher")
		}
	}

	go func() {
		for event := range events {
			// If proto, we generate the buf, it could trigger code changes
			proto := false
			if event.IsRelative && strings.HasSuffix(event.Path, ".proto") {
				proto = true
				p.PluginLogger.Info("runtime[starting] proto change detected: %v", event)
				err := helper.BufGenerate()
				if err != nil {
					p.PluginLogger.Info("runtime[starting] cannot generate proto: %v", err)
				}
			} else {
				p.PluginLogger.Info("runtime[starting] code change detected: %v", event)
			}
			if !proto && !strings.HasSuffix(event.Path, ".go") {
				continue
			}
			p.PluginLogger.Info("runtime[starting] relevant changes to go program detected: killing")
			err := p.Runner.Kill()
			if err != nil {
				p.PluginLogger.Info("runtime[starting] cannot kill go program: %v", err)
			}
			p.status = services.Stopped
		}
	}()

	p.Runner = &golanghelpers.Run{
		Dir:           dir,
		Args:          []string{"main.go"},
		ServiceLogger: plugins.NewServiceLogger(p.Name),
		PluginLogger:  p.PluginLogger,
		Debug:         p.Spec.Debug,
	}
	tracker, err := p.Runner.Run(context.Background())
	if err != nil {
		return nil, p.PluginLogger.WrapErrorf(err, "cannot run go")
	}

	p.status = services.Started
	return &runtimev1.StartResponse{
		Trackers: []*runtimev1.Tracker{tracker.Proto()},
	}, nil
}

func (p *Runtime) ServiceInformation(req *runtimev1.ServiceInformationRequest) (*runtimev1.ServiceInformationResponse, error) {
	return &runtimev1.ServiceInformationResponse{Status: p.status}, nil
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
		return nil, core.WrapErrorf(err, "cannot tidy go.mod")
	}
	return &runtimev1.SyncResponse{}, nil
}

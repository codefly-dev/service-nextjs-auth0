package main

import (
	"context"
	"github.com/hygge-io/hygge/pkg/configurations"
	"github.com/hygge-io/hygge/pkg/plugins"
	"github.com/hygge-io/hygge/pkg/plugins/helpers/code"
	golanghelpers "github.com/hygge-io/hygge/pkg/plugins/helpers/go"
	"github.com/hygge-io/hygge/pkg/plugins/services"
	runtimev1 "github.com/hygge-io/hygge/proto/services/runtime/v1"
	"path"
	"strings"
)

type Runtime struct {
	*Service
	Name   string
	Runner *golanghelpers.Run
	// internal
	status services.Status
}

func NewRuntime() *Runtime {
	return &Runtime{
		Service: NewService(),
	}
}

func (service *Runtime) Init(req *runtimev1.InitRequest) (*runtimev1.InitResponse, error) {
	defer service.PluginLogger.Catch()

	service.PluginLogger.SetDebug(req.Debug) // For developers
	service.Location = req.Location
	service.Name = req.Identity.Name

	service.status = services.Init

	err := configurations.LoadSpec(req.Spec, &service.Spec)
	if err != nil {
		return nil, service.PluginLogger.WrapErrorf(err, "factory[init]: cannot load spec")
	}
	service.PluginLogger.Info("runtime[init] initializing service at <%s> with Spec: %v", req.Location, service.Spec)

	return &runtimev1.InitResponse{}, nil
}

func (service *Runtime) Start(req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer service.PluginLogger.Catch()

	dir := path.Join(service.Location, Source)
	service.PluginLogger.Info("runtime[starting] go program in <%s> with spec: %v", dir, service.Spec)

	helper := golanghelpers.Go{Dir: path.Join(service.Location, Source)}

	// Exclude the proto
	events := make(chan code.Change)
	if service.Spec.Watch {
		service.PluginLogger.Info("runtime[starting] watching for changes in <%s>", dir)
		_, err := code.NewWatcher(service.PluginLogger, events, dir, []string{"."}, "api.proto")
		if err != nil {
			return nil, service.PluginLogger.WrapErrorf(err, "cannot create code watcher")
		}
	}

	go func() {
		for event := range events {
			// If proto, we generate the buf, it could trigger code changes
			proto := false
			if event.IsRelative && strings.HasSuffix(event.Path, ".proto") {
				proto = true
				service.PluginLogger.Info("runtime[starting] proto change detected: %v", event)
				err := helper.BufGenerate()
				if err != nil {
					service.PluginLogger.Error("runtime[starting] cannot generate proto: %v", err)
				}
			} else {
				service.PluginLogger.Info("runtime[starting] code change detected: %v", event)
			}
			if !proto && !strings.HasSuffix(event.Path, ".go") {
				continue
			}
			service.PluginLogger.Info("runtime[starting] relevant changes to go program detected: killing")
			err := service.Runner.Kill()
			if err != nil {
				service.PluginLogger.Error("runtime[starting] cannot kill go program: %v", err)
			}
			service.status = services.Stopped
		}
	}()

	service.Runner = &golanghelpers.Run{
		Dir:           dir,
		Args:          []string{"main.go"},
		ServiceLogger: plugins.NewServiceLogger(service.Name),
		PluginLogger:  service.PluginLogger,
		Debug:         service.Spec.Debug,
	}
	tracker, err := service.Runner.Run(context.Background())
	if err != nil {
		return nil, service.PluginLogger.WrapErrorf(err, "cannot run go")
	}

	service.status = services.Started
	return &runtimev1.StartResponse{
		Trackers: []*runtimev1.Tracker{tracker.Proto()},
	}, nil
}

func (service *Runtime) Status(req *runtimev1.StatusRequest) (*runtimev1.StatusResponse, error) {
	return &runtimev1.StatusResponse{Status: service.status}, nil
}

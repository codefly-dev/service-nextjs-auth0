package main

import (
	"context"
	"github.com/hygge-io/hygge/pkg/configurations"
	"github.com/hygge-io/hygge/pkg/plugins"
	golanghelpers "github.com/hygge-io/hygge/pkg/plugins/helpers/go"
	runtimev1 "github.com/hygge-io/hygge/proto/services/runtime/v1"
	"path"
)

type Runtime struct {
	*Service
	Logger *plugins.PluginLogger
}

func NewRuntime() *Runtime {
	return &Runtime{
		Logger:  plugins.NewPluginLogger(conf.Name()),
		Service: NewService(),
	}
}

func (service *Runtime) Init(req *runtimev1.InitRequest) (*runtimev1.InitResponse, error) {
	defer service.Logger.Catch()

	service.Logger.SetDebug(req.Debug) // For developers
	service.Location = req.Location

	err := configurations.LoadSpec(req.Spec, &service.Spec)
	if err != nil {
		return nil, service.Logger.WrapErrorf(err, "factory[init]: cannot load spec")
	}
	service.Logger.Info("factory[init] initializing service at <%s> with Spec: %s", req.Location, service.Spec)

	return &runtimev1.InitResponse{}, nil
}

func (service *Runtime) Start(req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer service.Logger.Catch()

	dir := path.Join(service.Location, Source)
	service.Logger.Info("runtime: starting go program in: %s", dir)
	g := &golanghelpers.Run{
		Dir:    dir,
		Args:   []string{"main.go"},
		Logger: service.Logger,
	}
	err := g.Run(context.Background())
	if err != nil {
		return nil, service.Logger.WrapErrorf(err, "cannot run go")
	}

	return &runtimev1.StartResponse{}, nil
}

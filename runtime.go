package main

import (
	"github.com/hygge-io/hygge-cli/pkg/platform/plugins"
	runtimev1 "github.com/hygge-io/hygge-cli/proto/services/runtime/v1"
)

type Runtime struct {
	Logger *plugins.PluginLogger
}

func NewRuntime() *Runtime {
	return &Runtime{
		Logger: plugins.NewPluginLogger(conf.Name()),
	}
}

func (r *Runtime) Init(req *runtimev1.InitRequest) (*runtimev1.InitResponse, error) {
	defer r.Logger.Catch()
	r.Logger.Debug("initializing service runtime in %s", req)
	return &runtimev1.InitResponse{}, nil
}

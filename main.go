package main

import (
	"github.com/hygge-io/hygge/pkg/configurations"
	"github.com/hygge-io/hygge/pkg/plugins"
	"github.com/hygge-io/hygge/pkg/plugins/services"
)

var conf = configurations.Plugin{
	Base:    "hygge-io/go-grpc",
	Version: "0.0.0",
}

func main() {
	plugins.Register(
		&plugins.Plugin{
			Configuration:  conf,
			Type:           plugins.ServiceFactory,
			Implementation: &services.ServiceFactoryPlugin{Factory: NewFactory()},
		},
		&plugins.Plugin{
			Configuration:  conf,
			Type:           plugins.ServiceRuntime,
			Implementation: &services.ServiceRuntimePlugin{Runtime: NewRuntime()},
		})
}

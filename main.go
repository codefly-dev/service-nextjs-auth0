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

type Service struct {
	Location string
	Spec     *Spec
}

func NewService() *Service {
	return &Service{
		Spec: &Spec{},
	}
}

const Source = "src"

type Spec struct {
	Src              string `mapstructure:"src"`
	WithDebugSymbols string `mapstructure:"with-debug-symbols"`
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

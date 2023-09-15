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
	PluginLogger *plugins.PluginLogger
	Location     string
	Spec         *Spec
}

func NewService() *Service {
	return &Service{
		PluginLogger: plugins.NewPluginLogger(conf.Name()),
		Spec:         &Spec{},
	}
}

const Source = "src"

type Spec struct {
	Src   string `mapstructure:"src"`
	Watch bool   `mapstructure:"watch"`
	Debug bool   `mapstructure:"debug"`
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

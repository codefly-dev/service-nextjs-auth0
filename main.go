package main

import (
	"github.com/hygge-io/hygge/pkg/configurations"
	"github.com/hygge-io/hygge/pkg/plugins"
	"github.com/hygge-io/hygge/pkg/plugins/services"
)

var conf = configurations.Plugin{
	Base:    "hygge-io/go-grpc",
	Version: "0.0.0",
	Kind:    configurations.PluginService,
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
		services.NewFactoryPlugin(conf, NewFactory()),
		services.NewRuntimePlugin(conf, NewRuntime()))
}

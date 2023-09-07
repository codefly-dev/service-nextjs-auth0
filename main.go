package main

import (
	"github.com/hygge-io/hygge-cli/pkg/services"
	"github.com/hygge-io/hygge-cli/proto/services/factory"
)

type Factory struct {
	Logger *services.PluginLogger
}

func New(name string) *Factory {
	return &Factory{
		Logger: services.NewPluginLogger(name),
	}
}

func (f *Factory) Create(req *factory.CreateRequest) (*factory.CreateResponse, error) {
	f.Logger.Info("Create me, mon ami")
	return &factory.CreateResponse{}, nil
}

func main() {
	name := "/Users/antoine/Development/hygge/hygge-cli/plugins/services/go-grpc/go-grpc-0.0.1.so"
	services.Serve(name, New(name))
}

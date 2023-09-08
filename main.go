package main

import (
	"embed"
	"fmt"
	"github.com/hygge-io/hygge-cli/pkg/platform/plugins"
	"github.com/hygge-io/hygge-cli/pkg/plugin"
	"github.com/hygge-io/hygge-cli/pkg/services"
	"github.com/hygge-io/hygge-cli/proto/services/factory/v1"
	"os/exec"
	"path"
	"strings"
)

type Factory struct {
	Logger *plugins.PluginLogger
}

const PluginName = "go-grpc/go-grpc"
const PluginVersion = "0.0.0"

func New(name string) *Factory {
	return &Factory{
		Logger: plugins.NewPluginLogger(name),
	}
}

type Proto struct {
	Package      string
	PackageAlias string
}

type CreateService struct {
	Name      string
	TitleName string
	Proto     Proto
	Go        GenerateInstructions
}

type GenerateInstructions struct {
	Package string
}

type PluginConfiguration struct {
	Name string
}

type CreateConfiguration struct {
	Name        string
	Destination string
	Namespace   string
	Service     CreateService
	Plugin      PluginConfiguration
}

func (f *Factory) Create(req *v1.CreateRequest) (*v1.CreateResponse, error) {
	f.Logger.Debug("creating service in %s", req.Instructions.Destination)
	service := strings.Title(req.Info.Name)
	proto := fmt.Sprintf("%s.v1", req.Info.Name)
	err := plugin.CopyTemplateDir(f.Logger, fs, req.Instructions.Destination, CreateConfiguration{
		Name:        req.Info.Name,
		Destination: req.Instructions.Destination,
		Namespace:   req.Info.Name,
		Service: CreateService{
			Name:      req.Info.Name,
			TitleName: service,
			Proto: Proto{
				Package:      proto,
				PackageAlias: strings.Replace(proto, ".", "_", -1),
			},
			Go: GenerateInstructions{
				Package: req.Info.Domain,
			},
		},
		Plugin: PluginConfiguration{
			Name: PluginName,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot init plugin for %s: %v", req.Info.Name, err)
	}
	src := path.Join(req.Instructions.Destination, "src")
	cmd := exec.Command("buf", "generate")
	cmd.Dir = src
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot run buf generate: %v", err)
	}
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = src
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("cannot run mod tidy: %v", err)
	}

	return &v1.CreateResponse{}, nil
}

//go:embed templates/*
var fs embed.FS

func Plugin() string {
	return fmt.Sprintf("%s:%s", PluginName, PluginVersion)
}

func main() {
	services.Serve(Plugin(), New(PluginName))
}

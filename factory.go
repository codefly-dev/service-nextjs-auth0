package main

import (
	"embed"
	"fmt"
	"github.com/hygge-io/hygge/pkg/configurations"
	"github.com/hygge-io/hygge/pkg/platform/plugins"
	"github.com/hygge-io/hygge/pkg/plugins/helpers"
	factoryv1 "github.com/hygge-io/hygge/proto/services/factory/v1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"path"
	"strings"
)

type Factory struct {
	Logger   *plugins.PluginLogger
	Identity *factoryv1.ServiceIdentity
}

const Source = "src"

type Spec struct {
	Src string `mapstructure:"src"`
}

func NewFactory() *Factory {
	return &Factory{
		Logger: plugins.NewPluginLogger(conf.Name()),
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

type CreateConfiguration struct {
	Name        string
	Destination string
	Namespace   string
	Service     CreateService
	Plugin      configurations.Plugin
}

func (f *Factory) Init(req *factoryv1.InitRequest) (*factoryv1.InitResponse, error) {
	defer f.Logger.Catch()

	f.Logger.Debug("factory>init: initializing service: %s", req)
	f.Identity = req.Identity
	return &factoryv1.InitResponse{}, nil
}

func (f *Factory) Create(req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer f.Logger.Catch()

	f.Logger.Debug("factory>create: req=%s", req)
	converter := cases.Title(language.English)

	// "Class name"
	title := converter.String(f.Identity.Name)
	// Proto package
	proto := fmt.Sprintf("%s.v1", f.Identity.Name)

	err := helpers.CopyTemplateDir(f.Logger, fs, req.Instructions.Destination, CreateConfiguration{
		Name:        f.Identity.Name,
		Destination: req.Instructions.Destination,
		Namespace:   f.Identity.Name,
		Service: CreateService{
			Name:      f.Identity.Name,
			TitleName: title,
			Proto: Proto{
				Package:      proto,
				PackageAlias: strings.Replace(proto, ".", "_", -1),
			},
			Go: GenerateInstructions{
				Package: f.Identity.Domain,
			},
		},
		Plugin: conf,
	})
	if err != nil {
		return nil, fmt.Errorf("factory>create: cannot copy from template dir %s for %s: %v", conf.Name(), f.Identity.Name, err)
	}

	conf, err := configurations.LoadServiceFromDir(req.Instructions.Destination)
	if err != nil {
		return nil, fmt.Errorf("factory>create: cannot load service configuration: %v", err)
	}
	f.Logger.Info("factory>create: loaded service configuration: %s", conf)
	spec := Spec{Src: Source}

	err = conf.AddSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("factory>create: cannot add spec: %v", err)
	}
	err = conf.SaveAtDir(req.Instructions.Destination)
	if err != nil {
		return nil, fmt.Errorf("factory>create: cannot save service configuration: %v", err)
	}

	helper := helpers.Go{Dir: path.Join(req.Instructions.Destination, Source)}

	err = helper.BufGenerate()
	if err != nil {
		return nil, fmt.Errorf("factory>create: go helper: cannot run buf generate: %v", err)
	}
	err = helper.ModTidy()
	if err != nil {
		return nil, fmt.Errorf("factory>create: go helper: cannot run mod tidy: %v", err)
	}

	return &factoryv1.CreateResponse{}, nil
}

func (f *Factory) Refresh(req *factoryv1.RefreshRequest) (*factoryv1.RefreshResponse, error) {
	defer f.Logger.Catch()

	f.Logger.Debug("refreshing service: %v", req)

	helper := helpers.Go{Dir: path.Join(req.Destination, Source)}

	err := helper.BufGenerate()
	if err != nil {
		return nil, fmt.Errorf("go helper: cannot run buf generate: %v", err)
	}
	err = helper.ModTidy()
	if err != nil {
		return nil, fmt.Errorf("go helper: cannot run mod tidy: %v", err)
	}

	return &factoryv1.RefreshResponse{}, nil
}

//go:embed templates/*
var fs embed.FS

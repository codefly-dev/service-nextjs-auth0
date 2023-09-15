package main

import (
	"embed"
	"fmt"
	"github.com/hygge-io/hygge/pkg/configurations"
	"github.com/hygge-io/hygge/pkg/plugins/helpers"
	golanghelpers "github.com/hygge-io/hygge/pkg/plugins/helpers/go"
	factoryv1 "github.com/hygge-io/hygge/proto/services/factory/v1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"path"
	"strings"
)

type Factory struct {
	*Service
	Identity *factoryv1.ServiceIdentity
	Location string
}

func NewFactory() *Factory {
	return &Factory{
		Service: NewService(),
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

func (service *Factory) Init(req *factoryv1.InitRequest) (*factoryv1.InitResponse, error) {
	defer service.PluginLogger.Catch()

	service.Identity = req.Identity
	service.Location = req.Location

	err := configurations.LoadSpec(req.Spec, &service.Spec)
	if err != nil {
		return nil, service.PluginLogger.WrapErrorf(err, "factory>init: cannot load spec")
	}
	service.PluginLogger.Debug("factory[init] initializing service with Spec: %v", service.Spec)

	return &factoryv1.InitResponse{}, nil
}

func (service *Factory) Create(req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer service.PluginLogger.Catch()

	converter := cases.Title(language.English)

	// "Class name"
	title := converter.String(service.Identity.Name)
	// Proto package
	proto := fmt.Sprintf("%s.v1", service.Identity.Name)

	err := helpers.CopyTemplateDir(service.PluginLogger, fs, service.Location, CreateConfiguration{
		Name:        service.Identity.Name,
		Destination: service.Location,
		Namespace:   service.Identity.Name,
		Service: CreateService{
			Name:      service.Identity.Name,
			TitleName: title,
			Proto: Proto{
				Package:      proto,
				PackageAlias: strings.Replace(proto, ".", "_", -1),
			},
			Go: GenerateInstructions{
				Package: service.Identity.Domain,
			},
		},
		Plugin: conf,
	})
	if err != nil {
		return nil, fmt.Errorf("factory>create: cannot copy from template dir %s for %s: %v", conf.Name(), service.Identity.Name, err)
	}

	conf, err := configurations.LoadServiceFromDir(service.Location)
	if err != nil {
		return nil, fmt.Errorf("factory>create: cannot load service configuration: %v", err)
	}
	service.PluginLogger.Info("factory[create] loaded service configuration: %v", conf)
	spec := Spec{Src: Source}

	err = conf.AddSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("factory>create: cannot add spec: %v", err)
	}
	err = conf.SaveAtDir(service.Location)
	if err != nil {
		return nil, fmt.Errorf("factory>create: cannot save service configuration: %v", err)
	}

	helper := golanghelpers.Go{Dir: path.Join(service.Location, Source)}

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

func (service *Factory) Refresh(req *factoryv1.RefreshRequest) (*factoryv1.RefreshResponse, error) {
	defer service.PluginLogger.Catch()

	service.PluginLogger.Debug("refreshing service: %v", req)

	helper := golanghelpers.Go{Dir: path.Join(req.Destination, Source)}

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

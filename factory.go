package main

import (
	"embed"
	"fmt"
	"github.com/hygge-io/hygge/pkg/configurations"
	"github.com/hygge-io/hygge/pkg/core"
	"github.com/hygge-io/hygge/pkg/plugins/helpers"
	golanghelpers "github.com/hygge-io/hygge/pkg/plugins/helpers/go"
	factoryv1 "github.com/hygge-io/hygge/proto/v1/services/factory"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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

func (p *Factory) Init(req *factoryv1.InitRequest) (*factoryv1.InitResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.Debugf("factory.init: %v", req)
	p.Identity = req.Identity
	p.Location = req.Location

	return &factoryv1.InitResponse{}, nil
}

func (p *Factory) Create(req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer p.PluginLogger.Catch()

	converter := cases.Title(language.English)

	// "Class name"
	title := converter.String(p.Identity.Name)
	// Proto package
	proto := fmt.Sprintf("%s.v1", p.Identity.Name)

	err := helpers.CopyTemplateDir(p.PluginLogger, fs, p.Location, CreateConfiguration{
		Name:        p.Identity.Name,
		Destination: p.Location,
		Namespace:   p.Identity.Name,
		Service: CreateService{
			Name:      p.Identity.Name,
			TitleName: title,
			Proto: Proto{
				Package:      proto,
				PackageAlias: strings.Replace(proto, ".", "_", -1),
			},
			Go: GenerateInstructions{
				Package: p.Identity.Domain,
			},
		},
		Plugin: conf,
	})

	if err != nil {
		return nil, fmt.Errorf("factory>create: cannot copy from template dir %s for %s: %v", conf.Name(), p.Identity.Name, err)
	}

	// Load default
	err = configurations.LoadSpec(req.Spec, &p.Spec, core.BaseLogger(p.PluginLogger))
	if err != nil {
		return nil, err
	}

	//	May override or check spec here
	spec, err := configurations.SerializeSpec(p.Spec)
	if err != nil {
		return nil, err
	}

	helper := golanghelpers.Go{Dir: p.Location}

	err = helper.BufGenerate()
	if err != nil {
		return nil, fmt.Errorf("factory>create: go helper: cannot run buf generate: %v", err)
	}
	err = helper.ModTidy(p.PluginLogger)
	if err != nil {
		return nil, fmt.Errorf("factory>create: go helper: cannot run mod tidy: %v", err)
	}

	return &factoryv1.CreateResponse{Spec: spec}, nil
}

//go:embed templates/*
var fs embed.FS

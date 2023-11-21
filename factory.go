package main

import (
	"embed"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/codefly-dev/cli/pkg/plugins/communicate"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	corev1 "github.com/codefly-dev/cli/proto/v1/core"
	v1 "github.com/codefly-dev/cli/proto/v1/services"
	factoryv1 "github.com/codefly-dev/cli/proto/v1/services/factory"
	"github.com/codefly-dev/core/configurations"
	"github.com/codefly-dev/core/shared"
)

type Factory struct {
	*Service

	// Communication
	create *communicate.ClientContext
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

type Readme struct {
	Summary string
}

type CreateConfiguration struct {
	Name        string
	Destination string
	Namespace   string
	Domain      string
	Service     CreateService
	Plugin      configurations.Plugin
	Readme      Readme
}

func (p *Factory) Init(req *v1.InitRequest) (*factoryv1.InitResponse, error) {
	defer p.PluginLogger.Catch()

	err := p.Base.Init(req, p.Settings)
	if err != nil {
		return nil, err
	}

	return &factoryv1.InitResponse{
		Version: p.Version(),
	}, nil
}

func (p *Factory) Create(req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer p.PluginLogger.Catch()

	p.ServiceLogger.Info("Creating service")

	create := CreateConfiguration{
		Name:      cases.Title(language.English, cases.NoLower).String(p.Identity.Name),
		Domain:    p.Identity.Domain,
		Namespace: p.Identity.Namespace,
		Readme:    Readme{Summary: p.Identity.Name},
	}

	ignores := []string{"node_modules", ".next", ".idea"}
	err := p.Templates(create, services.WithFactory(factory, ignores...), services.WithBuilder(builder))
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot copy and apply template")
	}

	out, err := shared.GenerateTree(p.Location, " ")
	if err != nil {
		return nil, err
	}
	p.PluginLogger.Info("tree: %s", out)

	return p.Base.Create(p.Settings)
}

func (p *Factory) Update(req *factoryv1.UpdateRequest) (*factoryv1.UpdateResponse, error) {
	defer p.PluginLogger.Catch()

	p.ServiceLogger.Info("Updating")

	err := p.Templates(nil, services.WithBuilder(builder))
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot copy and apply template")
	}

	return &factoryv1.UpdateResponse{}, nil
}

func (p *Factory) Sync(req *factoryv1.SyncRequest) (*factoryv1.SyncResponse, error) {
	defer p.PluginLogger.Catch()

	return &factoryv1.SyncResponse{}, nil
}

func (p *Factory) Build(req *factoryv1.BuildRequest) (*factoryv1.BuildResponse, error) {
	p.PluginLogger.Debugf("building docker image")

	return &factoryv1.BuildResponse{}, nil
}

func (p *Factory) Deploy(req *factoryv1.DeploymentRequest) (*factoryv1.DeploymentResponse, error) {
	return &factoryv1.DeploymentResponse{}, nil
}

func (p *Factory) Communicate(req *corev1.Engage) (*corev1.InformationRequest, error) {
	p.PluginLogger.DebugMe("factory communicate: %v", req)
	return p.Base.Communicate(req)
}

//go:embed templates/routes
var routes embed.FS

//go:embed templates/factory
var factory embed.FS

//go:embed templates/builder
var builder embed.FS

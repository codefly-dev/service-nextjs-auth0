package main

import (
	"embed"
	"fmt"
	"github.com/codefly-dev/cli/pkg/plugins/communicate"
	"github.com/codefly-dev/cli/pkg/plugins/endpoints"
	corev1 "github.com/codefly-dev/cli/proto/v1/core"

	golanghelpers "github.com/codefly-dev/cli/pkg/plugins/helpers/go"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	v1 "github.com/codefly-dev/cli/proto/v1/services"
	factoryv1 "github.com/codefly-dev/cli/proto/v1/services/factory"
	"github.com/codefly-dev/core/configurations"
	"github.com/codefly-dev/core/shared"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Factory struct {
	*Service

	create         *communicate.ClientContext
	createSequence *communicate.Sequence
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

	err := p.Base.Init(req)
	if err != nil {
		return nil, err
	}

	p.create, err = p.NewCreateCommunicate()
	if err != nil {
		return nil, err
	}

	channels, err := p.WithCommunications(services.NewChannel(communicate.Create, p.create))
	if err != nil {
		return nil, err
	}
	return &factoryv1.InitResponse{
		Version:  p.Version(),
		Channels: channels,
	}, nil
}

const Watch = "watch"

func (p *Factory) NewCreateCommunicate() (*communicate.ClientContext, error) {
	client, err := communicate.NewClientContext(p.Context(), communicate.Create)
	p.createSequence, err = client.NewSequence(
		client.NoOp(&corev1.Message{Message: "Thank you for choosing go-grpc plugin by codefly.dev"}),
		client.NewConfirm(&corev1.Message{Name: Watch, Message: "Code hot-reload (Recommended)?", Description: "Let codefly restart/resync your service when code changes are detected"}, true),
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (p *Factory) Create(req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer p.PluginLogger.Catch()

	// Make sure the communication for create has been done successfully
	if !p.create.Ready() {
		return nil, p.PluginLogger.Errorf("create: communication not ready")
	}

	p.Spec.Watch = p.create.Confirm(p.createSequence.Find(Watch)).Confirmed
	p.DebugMe("WATCHER %v", p.Spec.Watch)

	create := CreateConfiguration{
		Name:      cases.Title(language.English, cases.NoLower).String(p.Identity.Name),
		Domain:    p.Identity.Domain,
		Namespace: p.Identity.Namespace,
		Readme:    Readme{Summary: p.Identity.Name},
	}

	err := p.Templates(create, services.WithFactory(factory), services.WithBuilder(builder))
	if err != nil {
		return nil, err
	}

	out, err := shared.GenerateTree(p.Location, " ")
	if err != nil {
		return nil, err
	}
	p.PluginLogger.Info("tree: %s", out)

	err = p.CreateEndpoints()
	if err != nil {
		return nil, p.Wrapf(err, "cannot create endpoints")
	}

	helper := golanghelpers.Go{Dir: p.Location}

	err = helper.BufGenerate(p.PluginLogger)
	if err != nil {
		return nil, fmt.Errorf("factory>create: go helper: cannot run buf generate: %v", err)
	}
	err = helper.ModTidy(p.PluginLogger)
	if err != nil {
		return nil, fmt.Errorf("factory>create: go helper: cannot run mod tidy: %v", err)
	}

	return p.Base.Create(p.Spec, p.Endpoints...)
}

func (p *Factory) Update(req *factoryv1.UpdateRequest) (*factoryv1.UpdateResponse, error) {
	defer p.PluginLogger.Catch()

	p.ServiceLogger.Info("Updating")

	err := p.Base.Templates(nil, services.WithBuilder(builder))
	if err != nil {
		return nil, p.Wrapf(err, "cannot copy and apply template")
	}

	helper := golanghelpers.Go{Dir: p.Location}
	err = helper.Update(p.PluginLogger)
	if err != nil {
		return nil, fmt.Errorf("factory>update: go helper: cannot run update: %v", err)
	}
	return &factoryv1.UpdateResponse{}, nil
}

func (p *Factory) CreateEndpoints() error {
	grpc, err := endpoints.NewGrpcApi(&configurations.Endpoint{Name: "grpc"}, p.Local("api.proto"))
	if err != nil {
		return p.Wrapf(err, "cannot create grpc api")
	}
	p.Endpoints = append(p.Endpoints, grpc)

	rest, err := endpoints.NewRestApiFromOpenAPI(p.Context(), &configurations.Endpoint{Name: "rest", Public: true}, p.Local("api.swagger.json"))
	if err != nil {
		return p.Wrapf(err, "cannot create openapi api")
	}
	p.Endpoints = append(p.Endpoints, rest)
	return nil
}

//go:embed templates/factory
var factory embed.FS

//go:embed templates/builder
var builder embed.FS

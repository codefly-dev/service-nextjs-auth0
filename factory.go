package main

import (
	"embed"
	"fmt"
	"github.com/codefly-dev/cli/pkg/plugins/communicate"
	golanghelpers "github.com/codefly-dev/cli/pkg/plugins/helpers/go"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	corev1 "github.com/codefly-dev/cli/proto/v1/core"
	v1 "github.com/codefly-dev/cli/proto/v1/services"
	factoryv1 "github.com/codefly-dev/cli/proto/v1/services/factory"
	"github.com/codefly-dev/core/configurations"
	"github.com/codefly-dev/core/shared"
	"github.com/codefly-dev/core/templates"
	"strings"
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

func (p *Factory) NewCreateCommunicate() (*communicate.ClientContext, error) {
	client := communicate.NewClientContext(communicate.Create, p.Base.PluginLogger)
	err := client.NewSequence(
		client.NewConfirm(&corev1.Message{Name: "confirm"}, true),
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (p *Factory) Init(req *v1.InitRequest) (*factoryv1.InitResponse, error) {
	defer p.Base.PluginLogger.Catch()

	err := p.Base.Init(req, &p.Spec)
	if err != nil {
		return nil, err
	}

	p.create, err = p.NewCreateCommunicate()
	if err != nil {
		return nil, err
	}

	channels, err := p.Base.WithCommunications(services.NewChannel(communicate.Create, p.create))
	if err != nil {
		return nil, err
	}

	return &factoryv1.InitResponse{
		Version:  p.Base.Version(),
		Channels: channels,
	}, nil
}

func (p *Factory) Create(req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer p.Base.PluginLogger.Catch()

	// Make sure the communication for create has been done successfully
	if !p.create.Ready() {
		return nil, p.Base.PluginLogger.Errorf("create: communication not ready")
	}

	p.Base.ServiceLogger.Info("Creating service")

	create := CreateConfiguration{
		Name:      strings.Title(p.Base.Identity.Name),
		Domain:    p.Base.Identity.Domain,
		Namespace: p.Base.Identity.Namespace,
		Readme:    Readme{Summary: p.Base.Identity.Name},
	}

	// Templatize as usual
	err := templates.CopyAndApply(p.Base.PluginLogger, templates.NewEmbeddedFileSystem(factory), shared.NewDir("templates/factory"),
		shared.NewDir(p.Base.Location), create)
	if err != nil {
		return nil, p.Base.PluginLogger.Wrapf(err, "cannot copy and apply template")
	}

	err = templates.CopyAndApply(p.Base.PluginLogger, templates.NewEmbeddedFileSystem(builder), shared.NewDir("templates/builder"),
		shared.NewDir(p.Base.Local("builder")), nil)
	if err != nil {
		return nil, p.Base.PluginLogger.Wrapf(err, "cannot copy and apply template")
	}

	out, err := shared.GenerateTree(p.Base.Location, " ")
	if err != nil {
		return nil, err
	}
	p.Base.PluginLogger.Info("tree: %s", out)

	// Load default
	err = configurations.LoadSpec(req.Spec, &p.Spec, shared.BaseLogger(p.Base.PluginLogger))
	if err != nil {
		return nil, err
	}

	p.InitEndpoints()

	//	May override or check spec here
	spec, err := configurations.SerializeSpec(p.Spec)
	if err != nil {
		return nil, err
	}

	helper := golanghelpers.Go{Dir: p.Base.Location}

	err = helper.BufGenerate(p.Base.PluginLogger)
	if err != nil {
		return nil, fmt.Errorf("factory>create: go helper: cannot run buf generate: %v", err)
	}
	err = helper.ModTidy(p.Base.PluginLogger)
	if err != nil {
		return nil, fmt.Errorf("factory>create: go helper: cannot run mod tidy: %v", err)
	}

	grpc, err := services.NewGrpcApi(p.Base.Local("api.proto"))
	if err != nil {
		return nil, shared.Wrapf(err, "cannot create grpc api")
	}
	endpoint, err := services.WithApi(&p.GrpcEndpoint, grpc)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot add gRPC api to endpoint")
	}
	endpoints := []*corev1.Endpoint{endpoint}
	if p.RestEndpoint != nil {
		rest, err := services.NewOpenApi(p.Base.Local("adapters/v1/swagger/api.swagger.json"))
		if err != nil {
			return nil, shared.Wrapf(err, "cannot create REST api")
		}
		r, err := services.WithApi(p.RestEndpoint, rest)
		if err != nil {
			return nil, shared.Wrapf(err, "cannot add grpc api to endpoint")
		}
		endpoints = append(endpoints, r)
	}

	return &factoryv1.CreateResponse{
		Spec:      spec,
		Endpoints: endpoints,
	}, nil
}

func (p *Factory) Update(req *factoryv1.UpdateRequest) (*factoryv1.UpdateResponse, error) {
	defer p.Base.PluginLogger.Catch()

	p.Base.ServiceLogger.Info("Updating")

	err := templates.CopyAndApply(p.Base.PluginLogger, templates.NewEmbeddedFileSystem(builder), shared.NewDir("templates/builder"),
		shared.NewDir(p.Base.Local("builder")), nil)
	if err != nil {
		return nil, p.Base.PluginLogger.Wrapf(err, "cannot copy and apply template")
	}

	helper := golanghelpers.Go{Dir: p.Base.Location}
	err = helper.Update(p.Base.PluginLogger)
	if err != nil {
		return nil, fmt.Errorf("factory>update: go helper: cannot run update: %v", err)
	}
	return &factoryv1.UpdateResponse{}, nil
}

func (p *Factory) Communicate(req *corev1.Engage) (*corev1.InformationRequest, error) {
	p.Base.PluginLogger.DebugMe("factory communicate: %v", req)
	return p.Base.Communicate(req)
}

func (p *Service) InitEndpoints() {
	p.GrpcEndpoint = configurations.Endpoint{
		Name:        configurations.Grpc,
		Api:         &configurations.Api{Protocol: configurations.Grpc},
		Description: "Expose gRPC",
	}

	p.Base.PluginLogger.Debugf("initEndpoints: %v", p.Spec.CreateHttpEndpoint)
	if p.Spec.CreateHttpEndpoint {
		p.RestEndpoint = &configurations.Endpoint{
			Name:        configurations.Http,
			Api:         &configurations.Api{Protocol: configurations.Http, Framework: configurations.RestFramework},
			Description: "Expose REST",
		}
	}
}

//go:embed templates/factory
var factory embed.FS

//go:embed templates/builder
var builder embed.FS

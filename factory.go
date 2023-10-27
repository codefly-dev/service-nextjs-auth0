package main

import (
	"embed"
	"fmt"
	golanghelpers "github.com/codefly-dev/cli/pkg/plugins/helpers/go"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	corev1 "github.com/codefly-dev/cli/proto/v1/core"
	factoryv1 "github.com/codefly-dev/cli/proto/v1/services/factory"
	"github.com/codefly-dev/core/configurations"
	"github.com/codefly-dev/core/shared"
	"github.com/codefly-dev/core/templates"
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

func (p *Factory) Init(req *factoryv1.InitRequest) (*factoryv1.InitResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.Debugf("[factory::init] %v", req)
	p.Identity = req.Identity
	p.Location = req.Location

	return &factoryv1.InitResponse{RuntimeOptions: []*corev1.Option{
		services.NewRuntimeOption[bool]("watch", "🕵️Automatically restart on code changes", true),
		services.NewRuntimeOption[bool]("with-debug-symbols", "🕵️Run with debug symbols", true),
		services.NewRuntimeOption[bool]("create-rest-endpoint", "🚀Add automatically generated REST endpoint (useful for the API Gateway pattern)", true),
	}}, nil

}

func (p *Factory) Local(f string) string {
	return path.Join(p.Location, f)
}

func (p *Factory) Create(req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer p.PluginLogger.Catch()
	create := CreateConfiguration{
		Name:      strings.Title(p.Identity.Name),
		Domain:    p.Identity.Domain,
		Namespace: p.Identity.Namespace,
		Readme:    Readme{Summary: p.Identity.Name},
	}

	// Templatize as usual
	err := templates.CopyAndApply(p.PluginLogger, templates.NewEmbeddedFileSystem(factory), shared.NewDir("templates/factory"),
		shared.NewDir(p.Location), create)
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot copy and apply template")
	}

	err = templates.CopyAndApply(p.PluginLogger, templates.NewEmbeddedFileSystem(builder), shared.NewDir("templates/builder"),
		shared.NewDir(p.Local("builder")), nil)
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot copy and apply template")
	}

	out, err := shared.GenerateTree(p.Location, " ")
	if err != nil {
		return nil, err
	}
	p.PluginLogger.Info("tree: %s", out)

	// Load default
	err = configurations.LoadSpec(req.Spec, &p.Spec, shared.BaseLogger(p.PluginLogger))
	if err != nil {
		return nil, err
	}

	p.InitEndpoints()

	//	May override or check spec here
	spec, err := configurations.SerializeSpec(p.Spec)
	if err != nil {
		return nil, err
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

	grpc, err := services.NewGrpcApi(p.Local("api.proto"))
	if err != nil {
		return nil, shared.Wrapf(err, "cannot create grpc api")
	}
	endpoint, err := services.WithApi(&p.GrpcEndpoint, grpc)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot add gRPC api to endpoint")
	}
	endpoints := []*corev1.Endpoint{endpoint}
	if p.RestEndpoint != nil {
		rest, err := services.NewOpenApi(p.Local("adapters/v1/swagger/api.swagger.json"))
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
	helper := golanghelpers.Go{Dir: p.Location}
	err := helper.Update(p.PluginLogger)
	if err != nil {
		return nil, fmt.Errorf("factory>update: go helper: cannot run update: %v", err)
	}
	return &factoryv1.UpdateResponse{}, nil
}

func (p *Service) InitEndpoints() {
	p.GrpcEndpoint = configurations.Endpoint{
		Name:        configurations.Grpc,
		Api:         &configurations.Api{Protocol: configurations.Grpc},
		Description: "Expose gRPC",
	}

	p.PluginLogger.Debugf("initEndpoints: %v", p.Spec.CreateHttpEndpoint)
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

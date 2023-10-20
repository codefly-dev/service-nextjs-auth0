package main

import (
	"embed"
	"fmt"
	golanghelpers "github.com/codefly-dev/cli/pkg/plugins/helpers/go"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	factoryv1 "github.com/codefly-dev/cli/proto/v1/services/factory"
	"github.com/codefly-dev/core/configurations"
	"github.com/codefly-dev/core/shared"
	"github.com/codefly-dev/core/templates"
	"path"
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
	Service     CreateService
	Plugin      configurations.Plugin
	Readme      Readme
}

func (p *Factory) Init(req *factoryv1.InitRequest) (*factoryv1.InitResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.Debugf("[factory::init] %v", req)
	p.Identity = req.Identity
	p.Location = req.Location

	return &factoryv1.InitResponse{RuntimeOptions: []*factoryv1.RuntimeOption{
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

	err := templates.CopyAndApply(p.PluginLogger,
		templates.NewEmbeddedFileSystem(fs),
		shared.NewDir("templates/factory"),
		shared.NewDir(p.Location),
		CreateConfiguration{
			Readme: Readme{Summary: p.Identity.Name},
		})

	if err != nil {
		return nil, fmt.Errorf("[factory::create] cannot copy from templates dir %s for %s: %v", conf.Name(), p.Identity.Name, err)
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
	endpoints, err := services.WithCreateApis(grpc, p.GrpcEndpoint)
	if err != nil {
		return nil, shared.Wrapf(err, "cannot add gRPC api to endpoint")
	}

	if p.Spec.CreateHttpEndpoint {
		rest, err := services.NewOpenApi(p.Local("adapters/v1/swagger/api.swagger.json"))
		if err != nil {
			return nil, shared.Wrapf(err, "cannot create REST api")
		}
		other, err := services.WithCreateApis(rest, *p.RestEndpoint)
		if err != nil {
			return nil, shared.Wrapf(err, "cannot add grpc api to endpoint")
		}
		endpoints = append(endpoints, other...)
	}

	return &factoryv1.CreateResponse{
		Spec:      spec,
		Endpoints: endpoints,
	}, nil
}

//go:embed templates/*
var fs embed.FS

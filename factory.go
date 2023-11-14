package main

import (
	"embed"
	"os"

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
	seq    *communicate.Sequence
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
	client, err := communicate.NewClientContext(p.Context(), communicate.Create)
	if err != nil {
		return nil, err
	}
	p.seq, err = client.NewSequence()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (p *Factory) Init(req *v1.InitRequest) (*factoryv1.InitResponse, error) {
	defer p.PluginLogger.Catch()

	err := p.Base.Init(req)
	if err != nil {
		return nil, err
	}

	p.DebugMe("ARE YOU KIDDING ME???")
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

func (p *Factory) Create(req *factoryv1.CreateRequest) (*factoryv1.CreateResponse, error) {
	defer p.PluginLogger.Catch()

	// Make sure the communication for create has been done successfully
	if !p.create.Ready() {
		return nil, p.PluginLogger.Errorf("create: communication not ready")
	}

	//p.Spec.Watch =
	p.ServiceLogger.Info("Creating service")

	create := CreateConfiguration{
		Name:      cases.Title(language.English, cases.NoLower).String(p.Identity.Name),
		Domain:    p.Identity.Domain,
		Namespace: p.Identity.Namespace,
		Readme:    Readme{Summary: p.Identity.Name},
	}

	err := p.Templates(create, services.WithFactory(factory, "node_modules"), services.WithBuilder(builder))
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot copy and apply template")
	}

	// Handle _app.tsx because of golang embed limitations
	p.ServiceLogger.Info("Creating _app.tsx")
	err = os.WriteFile(p.Local("/pages/_app.tsx"), []byte(appTsx), 0o644)
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot save _app.tsx")
	}

	out, err := shared.GenerateTree(p.Location, " ")
	if err != nil {
		return nil, err
	}
	p.PluginLogger.Info("tree: %s", out)

	return p.Base.Create(p.Spec)
}

const appTsx = `
import { UserProvider } from "@auth0/nextjs-auth0/client";
import "../styles/globals.css";

export default function App({ Component, pageProps }) {
  const { user } = pageProps;

  return (
    <UserProvider user={user}>
      <Component {...pageProps} />
    </UserProvider>
  );
}
`

func (p *Factory) Update(req *factoryv1.UpdateRequest) (*factoryv1.UpdateResponse, error) {
	defer p.PluginLogger.Catch()

	p.ServiceLogger.Info("Updating")

	err := p.Templates(nil, services.WithBuilder(builder))
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot copy and apply template")
	}

	return &factoryv1.UpdateResponse{}, nil
}

func (p *Factory) Communicate(req *corev1.Engage) (*corev1.InformationRequest, error) {
	p.PluginLogger.DebugMe("factory communicate: %v", req)
	return p.Base.Communicate(req)
}

//go:embed templates/factory
var factory embed.FS

//go:embed templates/builder
var builder embed.FS

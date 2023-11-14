package main

import (
	"fmt"
	"os"

	"github.com/codefly-dev/cli/pkg/plugins/network"

	corev1 "github.com/codefly-dev/cli/proto/v1/core"

	"github.com/codefly-dev/cli/pkg/runners"

	dockerhelpers "github.com/codefly-dev/cli/pkg/plugins/helpers/docker"
	"github.com/codefly-dev/cli/pkg/plugins/services"
	servicev1 "github.com/codefly-dev/cli/proto/v1/services"
	runtimev1 "github.com/codefly-dev/cli/proto/v1/services/runtime"
)

type Runtime struct {
	*Service
	Runner *runners.Runner
}

func NewRuntime() *Runtime {
	return &Runtime{
		Service: NewService(),
	}
}

func (p *Runtime) Init(req *servicev1.InitRequest) (*runtimev1.InitResponse, error) {
	defer p.PluginLogger.Catch()

	err := p.Base.Init(req)
	if err != nil {
		return p.Base.RuntimeInitResponseError(err)
	}

	return p.Base.RuntimeInitResponse(p.Endpoints)
}

func (p *Runtime) Configure(req *runtimev1.ConfigureRequest) (*runtimev1.ConfigureResponse, error) {
	defer p.PluginLogger.Catch()

	return &runtimev1.ConfigureResponse{
		Status: services.ConfigureSuccess(),
	}, nil
}

func (p *Runtime) Start(req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer p.PluginLogger.Catch()

	p.DebugMe("I CAN START")

	p.PluginLogger.TODO("CLI also has a runner, make sure we only have one if possible")

	envs := os.Environ()
	nws, err := network.ConvertToEnvironmentVariables(req.NetworkMappings)
	if err != nil {
		return nil, p.Wrapf(err, "cannot convert network mappings")
	}
	for _, n := range nws {
		envs = append(envs, fmt.Sprintf("NEXT_PUBLIC_%s", n))
	}

	// Create the .env.local file
	err = os.WriteFile(p.Local(".env.local"), []byte(EnvLocal), 0644)
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot write .env.local file")
	}

	// Add the group
	p.Runner = &runners.Runner{
		Name:          p.Service.Identity.Name,
		Bin:           "npm",
		Args:          []string{"run", "dev"},
		Envs:          envs,
		PluginLogger:  p.PluginLogger,
		ServiceLogger: p.ServiceLogger,
		Dir:           p.Location,
		Debug:         p.Debug,
	}
	err = p.Runner.Init(p.Context())
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot start service")
	}
	//p.Runner.Wait = true
	tracker, err := p.Runner.Run(p.Context())
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot start go program")
	}

	return &runtimev1.StartResponse{
		Status:   services.StartSuccess(),
		Trackers: []*runtimev1.Tracker{tracker.Proto()},
	}, nil
}

const EnvLocal = `
AUTH0_ISSUER_BASE_URL=https://dev-4c24vdpgjj3eyqmy.us.auth0.com
AUTH0_CLIENT_ID=W0BIdRDyyBMzp8YVfEc5BHoM7PchdGJM
AUTH0_CLIENT_SECRET=EPDjeP2bCYf-RLYd_NmIX_7DUyYHTrCcuSkn5F-KpLPMznx-ZzzkVkOT5KgUi-85
AUTH0_AUDIENCE=https://codefly.ai
AUTH0_BASE_URL=http://localhost:3000
AUTH0_SECRET=3d39be6e671cb5656d1b3b7bca6c9e49e160cc3375d171f6e777bc76a5b35bb8`

func (p *Runtime) Information(req *runtimev1.InformationRequest) (*runtimev1.InformationResponse, error) {
	return &runtimev1.InformationResponse{Status: p.Status}, nil
}

func (p *Runtime) Stop(req *runtimev1.StopRequest) (*runtimev1.StopResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.Debugf("stopping service")

	err := p.Base.Stop()
	if err != nil {
		return nil, err
	}
	return &runtimev1.StopResponse{}, nil
}

func (p *Runtime) Sync(req *runtimev1.SyncRequest) (*runtimev1.SyncResponse, error) {
	defer p.PluginLogger.Catch()

	return &runtimev1.SyncResponse{}, nil
}

func (p *Runtime) Build(req *runtimev1.BuildRequest) (*runtimev1.BuildResponse, error) {
	p.PluginLogger.Debugf("building docker image")
	builder, err := dockerhelpers.NewBuilder(dockerhelpers.BuilderConfiguration{
		Root:  p.Location,
		Image: p.Identity.Name,
		Tag:   p.Configuration.Version,
	})
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot create builder")
	}
	builder.WithLogger(p.PluginLogger)
	_, err = builder.Build()
	if err != nil {
		return nil, p.PluginLogger.Wrapf(err, "cannot build image")
	}
	return &runtimev1.BuildResponse{}, nil
}

func (p *Runtime) Deploy(req *runtimev1.DeploymentRequest) (*runtimev1.DeploymentResponse, error) {
	return &runtimev1.DeploymentResponse{}, nil
}

func (p *Runtime) Communicate(req *corev1.Engage) (*corev1.InformationRequest, error) {
	return p.Base.Communicate(req)
}

/* Details

 */

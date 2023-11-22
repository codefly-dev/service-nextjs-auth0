package main

import (
	"fmt"
	"os"

	"github.com/codefly-dev/cli/pkg/plugins/network"

	corev1 "github.com/codefly-dev/cli/proto/v1/core"

	"github.com/codefly-dev/cli/pkg/runners"

	"github.com/codefly-dev/cli/pkg/plugins/helpers/code"
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

	err := p.Base.Init(req, p.Settings)
	if err != nil {
		return p.Base.RuntimeInitResponseError(err)
	}

	return p.Base.RuntimeInitResponse(p.Endpoints)
}

func (p *Runtime) Configure(req *runtimev1.ConfigureRequest) (*runtimev1.ConfigureResponse, error) {
	defer p.PluginLogger.Catch()

	if p.Settings.Watch {
		conf := services.NewWatchConfiguration([]string{"."}, "service.codefly.yaml")
		err := p.SetupWatcher(conf, p.EventHandler)
		if err != nil {
			p.PluginLogger.Warn("error in watcher")
		}
	}

	return &runtimev1.ConfigureResponse{
		Status: services.ConfigureSuccess(),
	}, nil
}

func (p *Runtime) Start(req *runtimev1.StartRequest) (*runtimev1.StartResponse, error) {
	defer p.PluginLogger.Catch()

	p.PluginLogger.TODO("CLI also has a runner, make sure we only have one if possible")

	envs := os.Environ()
	nws, err := network.ConvertToEnvironmentVariables(req.NetworkMappings)
	if err != nil {
		return nil, p.Wrapf(err, "cannot convert network mappings")
	}
	for _, n := range nws {
		envs = append(envs, fmt.Sprintf("NEXT_PUBLIC_%s", n))
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

func (p *Runtime) Communicate(req *corev1.Engage) (*corev1.InformationRequest, error) {
	return p.Base.Communicate(req)
}

/* Details

 */

func (p *Runtime) EventHandler(event code.Change) error {
	p.PluginLogger.DebugMe("got an event: %v", event)
	p.ServiceLogger.Info("-> Detected working code changes: restarting")
	p.PluginLogger.DebugMe("detected working code changes: restarting")
	p.WantRestart()
	return nil
}

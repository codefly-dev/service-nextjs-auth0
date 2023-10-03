package main

import (
	"context"
	codefly "github.com/codefly-ai/sdk/go"
	"github.com/hygge-io/go-grpc/base/adapters"
)

func main() {
	config := &adapters.Configuration{}
	config.EndpointGrpc = codefly.Endpoint("{{.Endpoint.Name}}::grpc").WithDefault(":10000").Host()
	if codefly.Value("{{.Endpoint.EnableHttp}}") {
		config.EndpointHttp = codefly.Endpoint("{{.Endpoint.Name}}::http").WithDefault(":10001").Host()
	}
	server, err := adapters.NewServer(config)
	if err != nil {
		panic(err)
	}
	err = server.Start(context.Background())
	if err != nil {
		panic(err)
	}
}

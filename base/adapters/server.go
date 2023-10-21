package adapters

import (
	"context"
	"fmt"
)

type Server struct {
	grpc *GrpcServer
	rest *RestServer
}

func NewServer(config *Configuration) (*Server, error) {

	grpc, err := NewGrpServer(config)
	if err != nil {
		return nil, err
	}
	var rest *RestServer
	if config.EndpointHttp != "" {
		rest, err = NewRestServer(config)
		if err != nil {
			return nil, err
		}
	}
	return &Server{
		grpc: grpc,
		rest: rest,
	}, nil
}

func (server *Server) Start(ctx context.Context) error {
	if server.rest != nil {
		go func() {
			err := server.rest.Run(ctx)
			if err != nil {
				panic(err)
			}
		}()
	}
	return server.grpc.Run(ctx)
}

func (server *Server) Stop() {
	fmt.Println("Stopping server...")
	server.grpc.gRPC.GracefulStop()
}

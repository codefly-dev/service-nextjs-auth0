package adapters

import (
	"context"
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
	rest, err := NewRestServer(config)
	if err != nil {
		return nil, err
	}
	return &Server{
		grpc: grpc,
		rest: rest,
	}, nil
}

func (server *Server) Start(ctx context.Context) error {
	go func() {
		err := server.rest.Run(ctx)
		if err != nil {
			panic(err)
		}
	}()
	return server.grpc.Run(ctx)
}

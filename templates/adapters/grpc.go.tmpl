package adapters

import (
	"context"
	"fmt"
	gen "github.com/hygge-io/go-grpc/base/adapters/v1"
	"google.golang.org/grpc"
	"net"
)

type Configuration struct {
	EndpointGrpc string
	EndpointHttp string
}

type GrpcServer struct {
	gen.UnimplementedWebServer
	configuration *Configuration
	gRPC          *grpc.Server
}

func (s *GrpcServer) Version(ctx context.Context, req *gen.VersionRequest) (*gen.VersionResponse, error) {
	return &gen.VersionResponse{
		Version: "v1",
	}, nil
}

func NewGrpServer(c *Configuration) (*GrpcServer, error) {
	grpcServer := grpc.NewServer()
	s := GrpcServer{
		configuration: c,
		gRPC:          grpcServer,
	}
	gen.RegisterWebServer(grpcServer, &s)
	return &s, nil
}

func (s *GrpcServer) Run(ctx context.Context) error {
	fmt.Println("Starting gRPC server at", s.configuration.EndpointGrpc)
	lis, err := net.Listen("tcp", s.configuration.EndpointGrpc)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	if err := s.gRPC.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %s", err)
	}
	return nil
}

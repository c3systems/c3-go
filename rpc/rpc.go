package rpc

import (
	"errors"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"

	context "golang.org/x/net/context"

	loghooks "github.com/c3systems/c3-go/log/hooks"
	"github.com/c3systems/c3-go/node/store"
	pb "github.com/c3systems/c3-go/rpc/pb"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	// ErrMethodNotSupported ...
	ErrMethodNotSupported = errors.New("method not supported")
)

const port = ":5005"

// RPC ...
type RPC struct {
	mempool store.Interface
}

// Config ...
type Config struct {
	Mempool store.Interface
}

// Server ...
type Server struct {
	service *RPC
}

// New ...
func New(cfg *Config) *RPC {
	listen, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal(err)
	}

	svc := &RPC{
		mempool: cfg.Mempool,
	}

	grpcServer := grpc.NewServer()
	pb.RegisterC3Server(grpcServer, &Server{
		service: svc,
	})
	reflection.Register(grpcServer)
	grpcServer.Serve(listen)

	log.Printf("[rpc] server running on port %s", port)

	return svc
}

// Send ...
func (s *Server) Send(ctx context.Context, r *pb.Request) (*pb.Response, error) {
	method := strings.ToLower(r.Method)
	result, err := s.handleRequest(method, r)
	if err != nil {
		log.Fatal(err)
	}

	return &pb.Response{
		Jsonrpc: r.Jsonrpc,
		Id:      r.Id,
		Result:  result,
	}, nil
}

// handleRequest ...
func (s *Server) handleRequest(method string, r *pb.Request) (*any.Any, error) {
	switch method {
	case "c3_ping":
		return ptypes.MarshalAny(s.service.ping())
	case "c3_latestblock":
		return ptypes.MarshalAny(s.service.latestBlock())
	default:
		return nil, ErrMethodNotSupported
	}
}

func init() {
	log.AddHook(loghooks.ContextHook{})
}

package grpcdelivery

import (
	"context"
	"encoding/json"

	"github.com/example/fintech-core-api/services/auth/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

const serviceName = "fintech.auth.v1.AuthService"

type jsonCodec struct{}

func (jsonCodec) Name() string {
	return "json"
}

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type ValidateTokenRequest struct {
	Token string `json:"token"`
}

type ValidateTokenResponse struct {
	Valid  bool   `json:"valid"`
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Error  string `json:"error,omitempty"`
}

type AuthServiceServer interface {
	ValidateToken(context.Context, *ValidateTokenRequest) (*ValidateTokenResponse, error)
}

type AuthServer struct {
	auth *usecase.AuthUsecase
}

func NewAuthServer(auth *usecase.AuthUsecase) *AuthServer {
	return &AuthServer{auth: auth}
}

func (s *AuthServer) ValidateToken(ctx context.Context, req *ValidateTokenRequest) (*ValidateTokenResponse, error) {
	claims, err := s.auth.ValidateToken(ctx, req.Token)
	if err != nil {
		return &ValidateTokenResponse{Valid: false, Error: "invalid token"}, nil
	}
	return &ValidateTokenResponse{
		Valid:  true,
		UserID: claims.UserID,
		Role:   claims.Role,
	}, nil
}

func RegisterAuthService(server *grpc.Server, service AuthServiceServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*AuthServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "ValidateToken",
				Handler:    validateTokenHandler,
			},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "proto/auth/auth.proto",
	}, service)
}

func validateTokenHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidateTokenRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuthServiceServer).ValidateToken(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/" + serviceName + "/ValidateToken",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuthServiceServer).ValidateToken(ctx, req.(*ValidateTokenRequest))
	}
	return interceptor(ctx, in, info, handler)
}

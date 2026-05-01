package grpcdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/example/fintech-core-api/services/transaction/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
)

const authServiceName = "fintech.auth.v1.AuthService"

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

type AuthClient struct {
	conn    *grpc.ClientConn
	timeout time.Duration
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

func NewAuthClient(addr string, timeout time.Duration) (*AuthClient, error) {
	conn, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	)
	if err != nil {
		return nil, err
	}
	return &AuthClient{conn: conn, timeout: timeout}, nil
}

func (c *AuthClient) Close() error {
	return c.conn.Close()
}

func (c *AuthClient) ValidateToken(ctx context.Context, token string) (domain.Actor, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var resp ValidateTokenResponse
	err := c.conn.Invoke(ctx, "/"+authServiceName+"/ValidateToken", &ValidateTokenRequest{Token: token}, &resp, grpc.ForceCodec(jsonCodec{}))
	if err != nil {
		return domain.Actor{}, err
	}
	if !resp.Valid {
		return domain.Actor{}, errors.New("invalid token")
	}
	return domain.Actor{UserID: resp.UserID, Role: resp.Role}, nil
}
